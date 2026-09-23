package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/linguaquest/server/internal/domain"
)

// 难度配比是独立审核后的硬门禁。模型偶尔会生成整体质量合格、但
// BASIC/ADVANCED 只差一题的版本；给定向修题更多机会，避免把可修复内容
// 直接退回用户。每一轮仍必须重新通过题目盲审和难度审核。
const maxListeningQuestionAttempts = 5

// 一次原文修订协议耗尽后允许全新换源；预算需覆盖两份原文的生成与审核，
// 同时仍受后台任务统一的总超时限制。
const listeningSourcePreparationTimeout = 18 * time.Minute

const listeningDifficultyReviewTimeout = 7 * time.Minute

func useBindingAcademicBlueprint(part int, band float64) bool {
	// 只有学术 Part 3/4 的 7.0–8.0 才需要绑定高阶考点蓝图。
	// Part 1/2 是日常场景，不能套用学术双事实蓝图；6.5 的学术段
	// 仍保留完整原文、盲审和独立难度审核，但取消额外蓝图往返，
	// 避免多个模型规划请求把后台任务耗尽。
	return part >= 3 && part <= 4 && band >= 7.0
}

// 单段专项沿用整卷的固定原文、盲审和题目校验，不生成阅读或写作。
func (g *OpenAIGenerator) GenerateListeningTraining(ctx context.Context, part int, band float64) (domain.MockExamSection, error) {
	if part < 1 || part > 4 || !validMockExamBand(band) {
		return domain.MockExamSection{}, errors.New("invalid listening part or target band")
	}
	guidance := listeningDifficultyGuidance(band)
	spec := mockExamSpecs[part-1]
	var source mockExamSource
	var preparedPlan *listeningQuestionPlan
	useBlueprint := useBindingAcademicBlueprint(part, band)
	sourceAttempts := 1
	if useBlueprint {
		sourceAttempts = 2
	}
	for sourceAttempt := 1; sourceAttempt <= sourceAttempts; sourceAttempt++ {
		sourceGuidance := guidance
		if sourceAttempt > 1 {
			sourceGuidance += "\nThe previous complete source passed language review but could not support a viable high-band blueprint. Write a genuinely different complete source with natural, distributed alternatives and qualifications; do not directly announce each combined resolution in one sentence."
		}
		// 高阶原稿可能需要一次完整修订；所有阶段仍受后台任务总上下文约束。
		sourceCtx, sourceCancel := context.WithTimeout(ctx, listeningSourcePreparationTimeout)
		generated, err := g.prepareListeningSource(sourceCtx, spec, band, sourceGuidance)
		sourceCancel()
		if err != nil {
			// 保存失败原稿供管理员诊断，未设置审批标记且前端不会展示失败正文。
			return domain.MockExamSection{Key: spec.key, TargetBand: band, PaperVersion: mockExamPaperVersion, Title: generated.Title, AudioScript: generated.AudioScript}, fmt.Errorf("听力原文生成失败: %w", err)
		}
		source = generated
		if !useBlueprint {
			break
		}
		planCtx, planCancel := context.WithTimeout(ctx, 10*time.Minute)
		plan, planErr := g.planHighBandListeningQuestions(planCtx, source, part, band, "")
		planCancel()
		if planErr == nil {
			preparedPlan = &plan
			break
		}
		if listeningPlanServiceFailure(planErr) {
			return domain.MockExamSection{}, fmt.Errorf("高阶听力考点规划失败: %w: %w", ErrListeningReviewUnavailable, planErr)
		}
		if sourceAttempt == sourceAttempts || ctx.Err() != nil {
			return domain.MockExamSection{}, fmt.Errorf("高阶听力原稿无法形成合格考点蓝图: %w", planErr)
		}
		log.Printf("listening_training part=%d band=%.1f source_attempt=%d stage=blueprint_rejected", part, band, sourceAttempt)
	}
	var lastSection domain.MockExamSection
	var lastErr error
	feedback, previous := "", ""
	var fixed map[int]domain.QuizQuestion
	// 最多三轮定向修订；审核失败只提供固定诊断给出题器，绝不降低门槛或循环到“同意”为止。
	for attempt := 1; attempt <= maxListeningQuestionAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return lastSection, err
		}
		// 高阶修题可能包含一次本地格式纠正和一次完整盲审，模型高延迟时六分钟不足。
		activeGuidance := guidance
		activePlan := preparedPlan
		if useBlueprint && activePlan == nil {
			planCtx, planCancel := context.WithTimeout(ctx, 10*time.Minute)
			plan, planErr := g.planHighBandListeningQuestions(planCtx, source, part, band, feedback)
			planCancel()
			if planErr != nil {
				if listeningPlanServiceFailure(planErr) {
					return lastSection, fmt.Errorf("高阶听力考点规划失败: %w: %w", ErrListeningReviewUnavailable, planErr)
				}
				return lastSection, fmt.Errorf("高阶听力考点规划失败: %w", planErr)
			}
			activePlan = &plan
			// 后续难度修订必须继续绑定同一蓝图；否则第二轮会退化为
			// 无蓝图整套出题，丢失题型、顺序和考点约束。
			preparedPlan = activePlan
			fixed = nil
		}
		if activePlan != nil {
			activeGuidance += "\n" + listeningQuestionPlanGuidance(*activePlan)
		}
		questionCtx, questionCancel := context.WithTimeout(ctx, 10*time.Minute)
		section, err := g.generateMockExamQuestionsWithFixedItemsAndPlan(questionCtx, mockExamSpecs[part-1], band, source, listeningTrainingQuestionPrompt(part, band), feedback, previous, fixed, activePlan, activeGuidance)
		questionCancel()
		if err != nil {
			// 当前蓝图无法稳定落地时，下一轮必须换一份完整蓝图；
			// 只有难度审核失败才沿用蓝图做定向修题。
			if useBlueprint && strings.Contains(err.Error(), "binding high-band plan") {
				preparedPlan = nil
				fixed = nil
				previous = ""
				lastErr = err
				if attempt < maxListeningQuestionAttempts {
					log.Printf("listening_training part=%d band=%.1f stage=question_blueprint_replan", part, band)
					continue
				}
			}
			if errors.Is(err, errMockExamModelRequestFailed) {
				return section, fmt.Errorf("听力题目生成或质量审核失败: %w: %v", ErrListeningReviewUnavailable, err)
			}
			return section, fmt.Errorf("听力题目生成或质量审核失败: %w", err)
		}
		lastSection = section
		// 难度审核在响应结构矛盾时会重试一次，给两次独立审核共享四分钟预算。
		reviewCtx, reviewCancel := context.WithTimeout(ctx, listeningDifficultyReviewTimeout)
		err = g.reviewListeningDifficulty(reviewCtx, &section)
		reviewCancel()
		if err == nil {
			return section, nil
		} else {
			lastSection, lastErr = section, err
			// 服务不可用或响应损坏不是题目难度缺陷，不触发重出题。
			if errors.Is(err, ErrListeningReviewUnavailable) || errors.Is(err, ErrListeningReviewInvalid) || ctx.Err() != nil {
				return section, err
			}
			if attempt < maxListeningQuestionAttempts {
				feedback = listeningRevisionFeedback(section, err)
				fixed = listeningRevisionFixedItems(section)
				// 后续轮次继续使用首轮已通过确定性校验的同一蓝图，锁定
				// 题号、题型和证据区域，只局部替换弱题。不得重新生成蓝图，
				// 也不得丢弃蓝图后让修题跨越原有听力顺序。
				draft, _ := json.Marshal(mockExamDraft{Instructions: section.Instructions, Questions: section.Questions})
				previous = string(draft)
			}
		}
	}
	return lastSection, fmt.Errorf("听力专项经过%d轮生成与难度审核仍未通过: %w", maxListeningQuestionAttempts, lastErr)
}

// 只在审核确认题目有效、仅难度配比不足时保留强题；有语义拒绝则不锁题。
func listeningRevisionFixedItems(section domain.MockExamSection) map[int]domain.QuizQuestion {
	audit := section.ListeningDifficulty
	if audit == nil || !section.QualityApproved || audit.ContentHash != ListeningContentHash(section) || !audit.Review.Approved || audit.Review.Confidence != "HIGH" || len(audit.Review.Items) != len(section.Questions) {
		return nil
	}
	advanced := 0
	mechanisms := map[string]bool{}
	for _, item := range audit.Review.Items {
		if item.Level == "ADVANCED" {
			advanced++
		}
		for _, mechanism := range item.Mechanisms {
			if mechanism != "detail" {
				mechanisms[mechanism] = true
			}
		}
	}
	profile := listeningProfileForSection(section)
	// 修订按内部高阶设计目标释放题位，而不是只补到正式最低线；否则审核降级后没有安全余量。
	upgrades, downgrades := listeningAuthoringAdvanced(section.TargetBand)-advanced, advanced-profile.maxAdvanced
	needsVariety := len(mechanisms) < 3
	fixed := map[int]domain.QuizQuestion{}
	for i, item := range audit.Review.Items {
		q := section.Questions[i]
		if item.Number != i+1 || mockExamCanonicalAnswer(q, q.AnswerKey) == "" || mockExamCanonicalAnswer(q, item.Answer) != mockExamCanonicalAnswer(q, q.AnswerKey) || !mockExamExactEvidence(section.AudioScript, item.Evidence) {
			return nil
		}
		if item.Level == "BASIC" {
			continue
		}
		if item.Level == "STANDARD" && (upgrades > 0 || needsVariety) {
			upgrades--
			needsVariety = false
			continue
		}
		if item.Level == "ADVANCED" && downgrades > 0 {
			downgrades--
			continue
		}
		if item.Level != "STANDARD" && item.Level != "ADVANCED" {
			return nil
		}
		fixed[i] = q
	}
	return fixed
}

func listeningRevisionFeedback(section domain.MockExamSection, cause error) string {
	feedback := cause.Error()
	if section.ListeningDifficulty == nil {
		return feedback
	}
	review := section.ListeningDifficulty.Review
	basic := []int{}
	advanced := 0
	for _, item := range review.Items {
		if item.Level == "BASIC" {
			basic = append(basic, item.Number)
		}
		if item.Level == "ADVANCED" {
			advanced++
		}
	}
	profile := listeningProfileForSection(section)
	if strings.TrimSpace(review.Feedback) != "" {
		feedback += "\nREVIEWER SEMANTIC REJECTION: " + review.Feedback + " Replace every item explicitly named as overlapping, repeated, ambiguous or defective. Two questions still test the same underlying fact when one merely adds an incidental schedule step, condition or recap; such additions do not make the coverage distinct.\n"
	}
	feedback += fmt.Sprintf("\nREQUIRED CONTENT REVISION: independent review counted %d BASIC items at question numbers %v and %d ADVANCED. Allowed: at most %d BASIC and %d-%d ADVANCED. For the next draft author %d ADVANCED items to leave one review margin, while the formal release gate remains %d-%d. Replace weak tasks, not just their wording. Aim below the BASIC ceiling to allow reviewer disagreement. Preserve sound challenging items, but replace redundant direct-retrieval completions with uniquely supported paraphrase/correction tasks. Do NOT put the decisive correction or condition into the question itself, since that removes the listening operation. A cosmetic synonym or changing option letters alone is not a difficulty repair. If the source cannot support a better task, do not invent facts.\n", len(basic), basic, advanced, profile.maxBasic, profile.minAdvanced, profile.maxAdvanced, listeningAuthoringAdvanced(section.TargetBand), profile.minAdvanced, profile.maxAdvanced)
	data, _ := json.Marshal(review)
	return feedback + "Independent item-level review (untrusted): " + string(data)
}

func listeningTrainingQuestionPrompt(part int, band float64) string {
	// 专项训练只生成听力题目；避免把阅读/写作规则和整卷说明带入长请求。
	spec := mockExamSpecs[part-1]
	prompt := fmt.Sprintf(`Create section %s of an original IELTS listening practice: exactly 10 questions for %s. The frozen audio script is supplied separately. Return one JSON object only with title, instructions, audioScript, passage, questions, writingPrompts. Use meaningful English title and instructions, empty writingPrompts, and empty passage/audioScript; do not repeat the script. Each question must contain only type, question, options, answerKey and evidence. Every answer must be uniquely supported by a continuous verbatim quote of 4-60 words from the frozen script. Use chronological order, varied paraphrase and plausible distractors. Do not claim official IELTS equivalence.
Allowed types (exact lowercase names):
- multiple_choice: at least TWO items. Exactly four options ["A. full text", "B. full text", "C. full text", "D. full text"], single A-D letter answerKey. All distractors must be plausible alternatives about the SAME decision or phenomenon, similar in length/specificity, unambiguously wrong for a source-supported reason. Avoid absurd or absolute options.
- sentence_completion: at least TWO items. Empty options, exactly one literal ____ gap in a complete natural sentence. Include "NO MORE THAN THREE WORDS", "NO MORE THAN TWO WORDS" or "NO MORE THAN ONE WORD" in EACH question. answerKey is 1-3 consecutive verbatim source words within that limit. Put optional modifiers outside the gap; ensure no shorter or longer alternative fits. Never repeat the answer in the question.
- matching_information: optional, with 3-8 sequential letter-labelled full options and a single letter answerKey. Speaker options contain names ONLY, never the opinion being tested. All references must be identifiable in the script.
No true_false_not_given or unsupported question types. Number questions locally 1-10 if printed. Do not introduce any speaker or scene absent from the frozen source.`, spec.key, spec.form)
	prompt += "\nEach question must assess a distinct decision, claim or limitation. Do not ask the same underlying fact twice using different wording, even if one question adds an incidental detail."
	if band >= 6.5 {
		prompt += `
For this higher-target standalone set use exactly TWO sentence_completion items; use multiple_choice or genuine contrasting-opinion matching for the other eight. The completions must paraphrase the situation that identifies the answer, NOT copy the surrounding source sentence with one word removed. Do not supply the resolution in the question. Avoid "who said this?" speaker identification: merely recognising the name before a sentence is BASIC. Instead compare a speaker's initial proposal with the later qualification, or select which interpretation accounts for both the observed evidence and its limitation. Do not select a fact only because it is convenient to make a blank.`
		basic := listeningAuthoringBasic(band)
		advanced := listeningAuthoringAdvanced(band)
		if band == 6.5 && (part == 1 || part == 2) {
			advanced = 3
		}
		prompt += fmt.Sprintf(`
MANDATORY DIFFICULTY MIX: privately design and return exactly %d BASIC, %d STANDARD and %d ADVANCED items. The labels must describe the comprehension actually necessary, not a wish. Every ADVANCED item must require two distinct necessary non-detail mechanisms and plausible partial distractors; every STANDARD item must require exactly one non-detail mechanism. If a draft item is directly answerable from one explicit sentence, count it as BASIC and redesign it unless it is within the exact BASIC allowance. Do not return a merely acceptable question set with the wrong mix.`, basic, 10-basic-advanced, advanced)
		if band == 6.5 && (part == 1 || part == 2) {
			prompt += `
EVERYDAY 6.5 CONSTRUCTION CHECK: do not let all questions become direct retrieval. Privately map two multiple-choice positions to ADVANCED tasks that combine two facts from different turns: one plausible plan or preference introduced earlier and a later correction, schedule restriction, health limitation, availability condition or consequence. Build one partial distractor that satisfies the earlier fact but fails the later condition, and another that satisfies the later condition but fails the earlier fact. Do not use a name, date, fee, single reason, directly stated purpose or one-sentence answer as an advanced task. Use the remaining positions for distinct STANDARD paraphrase, correction, constraint or stance tasks; at most one completion may be BASIC, and the other completion must require resolving a hidden correction or condition before extracting the answer. This is an authoring target, not an approval result; the independent reviewer must classify the actual operations and the server gate remains unchanged.`
		}
	}
	if band >= 7.5 {
		profile := listeningProfile(band)
		prompt += fmt.Sprintf(`
HIGH-BAND AUTHORING CHECK: before returning JSON, privately classify the actual operation needed for every item under the supplied BASIC/STANDARD/ADVANCED rubric. Author exactly %d ADVANCED, %d STANDARD and %d BASIC items for this draft. Do not merely label them: each ADVANCED item must force the listener to combine two separately stated necessary facts, with each wrong option satisfying one but not both. STANDARD completion answers may be exact source words when the stem requires a real paraphrase, correction or condition to identify them uniquely. If another item asks only for an explicitly stated purpose, conclusion, reason, schedule, procedure or phrase without such processing, count it as BASIC and redesign it unless it is the single planned BASIC item. Do not use both completion items as direct-detail retrieval.`, profile.minAdvanced, 10-profile.minAdvanced-(profile.maxBasic-1), profile.maxBasic-1)
	}
	return prompt + `
Distribute correct multiple-choice positions across A-D without a predictable sequence. If there are at least four multiple-choice items, use at least three different correct letters and never put more than half (rounded up) in the same position. Reorder full options and update answerKey together; never change which content is correct.
Copy evidence directly, starting where the exact quoted words occur. Do not prepend a speaker name to a mid-turn extract or remove intervening words. Prefer a short complete decisive sentence over joining fragments from different turns.
STANDALONE TRAINING: question TYPE is not difficulty. Two completion items are required, but at most ONE may be a direct-detail BASIC item. The other completion must require the listener to resolve a correction or constraint before extracting consecutive source words; its prompt must not reveal that decisive correction or condition. Apply the supplied editorial profile across all ten items. Before drafting each item, identify the decisive information and alternatives: if a listener can copy one obvious phrase without resolving a correction, condition, paraphrase or viewpoint, count it as BASIC regardless of technical vocabulary. Definitions, directly stated reasons and single-sentence word spotting are BASIC, not high-band tasks. Do not call a directly stated explanation ADVANCED. For harder items, make BOTH mechanisms necessary and make each distractor plausible until the decisive qualification is heard. Never fabricate facts missing from the frozen source.
For ADVANCED questions, ask which plan, interpretation or option meets two actual source facts. Do not replace this with "what does the speaker recommend?" when the recommendation is explicitly stated. Each distractor must satisfy one necessary fact while contradicting the other, so only the correct option satisfies both; unrelated activities are invalid distractors. For STANDARD questions, alternate intention/attitude paraphrase, correction, and conditional limitations; do not repeat a "what must they do?" prerequisite pattern. Keep choices concise enough to read while listening.
Write questions in the order the decisive answers occur in the recording, including completion and matching groups. Make completion gaps grammatically unique: put optional modifiers outside the gap and use a restrictive ONE/TWO WORDS limit when appropriate. Supply a complete natural sentence, never a fragment such as "Alongside bubble wrap, the third material is ..." with an answer repeated in the prompt. Keep internal design notes out of the JSON.`
}

func ValidateListeningTraining(section domain.MockExamSection, part int) error {
	if part < 1 || part > 4 || !validMockExamBand(section.TargetBand) || section.PaperVersion != mockExamPaperVersion || !section.QualityApproved {
		return errors.New("listening training has not passed quality review")
	}
	if err := validateMockExamSection(section, mockExamSpecs[part-1]); err != nil {
		return fmt.Errorf("listening part %d: %w", part, err)
	}
	if err := validateListeningEditorialStructure(section); err != nil {
		return err
	}
	return ValidateListeningDifficulty(section)
}

func validateListeningEditorialStructure(section domain.MockExamSection) error {
	if err := validateListeningAnswerPositions(section); err != nil {
		return err
	}
	source := strings.Join(strings.Fields(section.AudioScript), " ")
	previousStart := -1
	for i, q := range section.Questions {
		quote := strings.Join(strings.Fields(q.Evidence), " ")
		start := strings.Index(source, quote)
		if quote == "" || start < 0 {
			return fmt.Errorf("question %d lacks chronological source evidence", i+1)
		}
		// 重叠证据可用于连续的关联题；完全回跳到更早内容则违背录音顺序。
		if start+len(quote) <= previousStart {
			return fmt.Errorf("question %d evidence occurs entirely before the preceding question; reorder questions to follow the recording", i+1)
		}
		if start > previousStart {
			previousStart = start
		}
	}
	return nil
}

// 无蓝图整套出题时，模型偶尔只把一两道题的证据放到了前面。
// 证据均已通过连续原文校验时，可以稳定地按录音位置重排整题；不修改题意、答案或证据，
// 也不触碰有冻结题位/蓝图的修订结果。重复引文或无法唯一定位时返回 false，交给模型重写。
var listeningQuestionReference = regexp.MustCompile(`(?i)\b(?:questions?|items?|q)\s*\d|\b(?:previous|preceding|following|next|above|below)\b`)

func orderListeningQuestionsByEvidence(section *domain.MockExamSection) bool {
	if section == nil || len(section.Questions) < 2 {
		return false
	}
	source := strings.Join(strings.Fields(section.AudioScript), " ")
	positions := make([]int, len(section.Questions))
	seen := map[int]bool{}
	for i, question := range section.Questions {
		text := candidateQuestionNumberPrefix.ReplaceAllString(strings.TrimSpace(question.Question), "") + " " + strings.Join(question.Options, " ")
		if listeningQuestionReference.MatchString(text) || !mockExamExactEvidence(section.AudioScript, question.Evidence) {
			return false
		}
		quote := strings.Join(strings.Fields(question.Evidence), " ")
		if quote == "" || strings.Count(source, quote) != 1 {
			return false
		}
		position := strings.Index(source, quote)
		if position < 0 || seen[position] {
			return false
		}
		seen[position] = true
		positions[i] = position
	}
	ordered := sort.SliceIsSorted(positions, func(i, j int) bool { return positions[i] < positions[j] })
	if ordered {
		return false
	}
	indices := make([]int, len(section.Questions))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool { return positions[indices[i]] < positions[indices[j]] })
	questions := make([]domain.QuizQuestion, len(section.Questions))
	for i, index := range indices {
		questions[i] = section.Questions[index]
	}
	section.Questions = questions
	draft := mockExamDraft{Questions: section.Questions}
	normalizeCandidateQuestionNumbers(&draft)
	return true
}

// 模型偶尔生成了合法的单空填空题，却省略 IELTS 常用的标准词数短语。
// 这是纯展示格式修正；不改题意、答案、证据或难度。
func normalizeListeningCompletionLimits(section *domain.MockExamSection) int {
	if section == nil {
		return 0
	}
	changed := 0
	for i := range section.Questions {
		question := &section.Questions[i]
		if question.Type != "sentence_completion" || strings.Count(question.Question, "____") != 1 || mockExamWordCount(question.AnswerKey) < 1 || mockExamWordCount(question.AnswerKey) > 3 {
			continue
		}
		upper := strings.ToUpper(question.Question)
		if strings.Contains(upper, "NO MORE THAN ONE WORD") || strings.Contains(upper, "NO MORE THAN TWO WORDS") || strings.Contains(upper, "NO MORE THAN THREE WORDS") {
			continue
		}
		question.Question = strings.TrimSpace(question.Question) + " NO MORE THAN THREE WORDS"
		changed++
	}
	return changed
}

// 这是防猜题的编辑检查，不是官方难度标准；不足四道选择题不强求均衡。
func validateListeningAnswerPositions(section domain.MockExamSection) error {
	counts := map[string]int{}
	total := 0
	for _, q := range section.Questions {
		if q.Type == "multiple_choice" {
			counts[mockExamCanonicalAnswer(q, q.AnswerKey)]++
			total++
		}
	}
	if total < 4 {
		return nil
	}
	for _, count := range counts {
		if len(counts) < 3 || count > (total+1)/2 {
			return errors.New("multiple-choice answer positions are concentrated; reorder complete options and update answer keys, using at least three different letters and no more than half (rounded up) of answers at one position; preserve correct content")
		}
	}
	return nil
}

// 只修正选择题位置偏置，不把已经通过内容审核的题目交给模型重写。
func rebalanceListeningAnswerPositions(section *domain.MockExamSection) bool {
	if section == nil || validateListeningAnswerPositions(*section) == nil {
		return false
	}
	for _, q := range section.Questions {
		if q.Type != "multiple_choice" {
			continue
		}
		answer := mockExamCanonicalAnswer(q, q.AnswerKey)
		if len(q.Options) != 4 || len(answer) != 1 || answer[0] < 'A' || answer[0] > 'D' {
			return false
		}
		for _, option := range q.Options {
			if len(mockExamOption.FindStringSubmatch(option)) != 3 {
				return false
			}
		}
	}

	// 每八题均衡覆盖 A-D；按原稿长度旋转，避免不同材料都呈现同一序列。
	pattern := []int{1, 3, 0, 2, 2, 0, 3, 1}
	offset := len([]rune(section.AudioScript)) % 4
	multipleChoiceIndex := 0
	for i := range section.Questions {
		q := &section.Questions[i]
		if q.Type != "multiple_choice" {
			continue
		}
		answer := mockExamCanonicalAnswer(*q, q.AnswerKey)
		current := int(answer[0] - 'A')
		target := (pattern[multipleChoiceIndex%len(pattern)] + offset) % 4
		q.Options[current], q.Options[target] = q.Options[target], q.Options[current]
		for optionIndex, option := range q.Options {
			text := strings.TrimSpace(mockExamOption.FindStringSubmatch(option)[2])
			q.Options[optionIndex] = fmt.Sprintf("%c. %s", 'A'+optionIndex, text)
		}
		q.AnswerKey = string(rune('A' + target))
		multipleChoiceIndex++
	}
	return validateListeningAnswerPositions(*section) == nil
}

// 局部修题只能重排未锁定选择题，防止格式修复破坏已通过独立审核的题。
func rebalanceMutableListeningAnswerPositions(section *domain.MockExamSection, fixed map[int]domain.QuizQuestion) bool {
	if section == nil || validateListeningAnswerPositions(*section) == nil {
		return false
	}
	counts := map[int]int{0: 0, 1: 0, 2: 0, 3: 0}
	mutable := []int{}
	for i, question := range section.Questions {
		if question.Type != "multiple_choice" {
			continue
		}
		answer := mockExamCanonicalAnswer(question, question.AnswerKey)
		if len(answer) != 1 || answer[0] < 'A' || answer[0] > 'D' || len(question.Options) != 4 {
			return false
		}
		if _, locked := fixed[i]; locked {
			counts[int(answer[0]-'A')]++
		} else {
			mutable = append(mutable, i)
		}
	}
	for _, index := range mutable {
		target := 0
		for candidate := 1; candidate < 4; candidate++ {
			if counts[candidate] < counts[target] {
				target = candidate
			}
		}
		question := &section.Questions[index]
		answer := mockExamCanonicalAnswer(*question, question.AnswerKey)
		current := int(answer[0] - 'A')
		question.Options[current], question.Options[target] = question.Options[target], question.Options[current]
		for optionIndex, option := range question.Options {
			match := mockExamOption.FindStringSubmatch(option)
			if len(match) != 3 {
				return false
			}
			question.Options[optionIndex] = fmt.Sprintf("%c. %s", 'A'+optionIndex, strings.TrimSpace(match[2]))
		}
		question.AnswerKey = string(rune('A' + target))
		counts[target]++
	}
	return validateListeningAnswerPositions(*section) == nil
}
