package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linguaquest/server/internal/domain"
)

const (
	paymentPending = "PENDING"
	paymentPaid    = "PAID"

	AICreditActionTheaterGeneration = "THEATER_GENERATION"
	AICreditActionReadingGeneration = "READING_GENERATION"
	AICreditActionWritingEvaluation = "WRITING_EVALUATION"
	AICreditActionWritingPrompt     = "WRITING_PROMPT"
	AICreditActionRoleplayTurn      = "ROLEPLAY_TURN"
	AICreditActionVoiceDesign       = "VOICE_DESIGN"
)

// BillingStore is deliberately separate from the legacy learning store so
// deployments can fail closed when a persistent billing implementation is unavailable.
type BillingStore interface {
	CreatePaymentOrder(order domain.PaymentOrder) (domain.PaymentOrder, error)
	GetPaymentOrder(orderID string, userID string) (domain.PaymentOrder, error)
	MarkPaymentOrderPaid(orderID string, providerTradeNo string, product domain.BillingProduct, paidAt time.Time) (domain.PaymentOrder, error)
	GetBillingStatus(userID string, now time.Time, freeDailyCredits int) (domain.BillingStatus, error)
	ConsumeCredits(userID string, activity string, sourceID string, amount int, now time.Time, freeDailyCredits int) (domain.BillingStatus, error)
	RefundCredits(userID string, activity string, sourceID string, amount int, now time.Time) error
	RecordMiniProgramUse(userID string, activity string, sourceID string, now time.Time) (int, error)
	RefundMiniProgramUse(userID string, activity string, sourceID string) error
	CountMiniProgramUses(userID string, now time.Time) (int, error)
}

type BillingOptions struct {
	OpenSourceEdition      bool
	MiniProgramEdition     bool
	Enabled                bool
	FreeDailyCredits       int
	MiniAdFreeDailyUses    int
	MiniProgramDailyAIUses int
	Timezone               string
	EpayGatewayURL         string
	EpayMerchantID         string
	EpayKey                string
	EpayNotifyURL          string
	DefaultChannel         string
	EpaySignatureMode      string
	AdProvider             string
	AdScriptURL            string
	AdCourseSlot           string
	AdLibrarySlot          string
	AdResultSlot           string
}

func aiCreditCosts() []domain.AICreditCost {
	return []domain.AICreditCost{
		{Action: AICreditActionReadingGeneration, Label: "阅读材料生成", Credits: 20, Description: "长篇阅读、词汇语法分析与多段 TTS 朗读"},
		{Action: AICreditActionVoiceDesign, Label: "角色音色设计", Credits: 12, Description: "音色设计与试听音频生成"},
		{Action: AICreditActionTheaterGeneration, Label: "AI 小剧场生成", Credits: 10, Description: "8 轮情境对话、测验与角色语音"},
		{Action: AICreditActionWritingEvaluation, Label: "写作 AI 评分", Credits: 6, Description: "语法、表达与结构多维评分"},
		{Action: AICreditActionWritingPrompt, Label: "写作主题生成", Credits: 1, Description: "按考试类型生成英文写作题目"},
		{Action: AICreditActionRoleplayTurn, Label: "角色对话单轮", Credits: 2, Description: "AI 续聊与语音回复"},
	}
}

func aiCreditAmount(action string) int {
	for _, cost := range aiCreditCosts() {
		if cost.Action == action {
			return cost.Credits
		}
	}
	return 0
}

type billingService struct {
	store    BillingStore
	options  BillingOptions
	public   string
	location *time.Location
}

func newBillingService(store BillingStore, options BillingOptions, publicURL string) *billingService {
	if options.FreeDailyCredits <= 0 {
		options.FreeDailyCredits = 20
	}
	if options.MiniAdFreeDailyUses <= 0 {
		options.MiniAdFreeDailyUses = 3
	}
	if options.MiniProgramDailyAIUses <= 0 {
		options.MiniProgramDailyAIUses = 20
	}
	if strings.TrimSpace(options.DefaultChannel) == "" {
		options.DefaultChannel = "alipay"
	}
	location, err := time.LoadLocation(strings.TrimSpace(options.Timezone))
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	return &billingService{store: store, options: options, public: strings.TrimRight(publicURL, "/"), location: location}
}

func billingProducts() []domain.BillingProduct {
	return []domain.BillingProduct{
		{Code: "monthly-lite", Name: "轻享月卡", Kind: "SUBSCRIPTION", AmountCents: 990, CreditAllowance: 800, PeriodDays: 30, AdsFree: true, Description: "每 30 天 800 AI 点数，去广告"},
		{Code: "monthly-plus", Name: "进阶月卡", Kind: "SUBSCRIPTION", AmountCents: 1990, CreditAllowance: 2000, PeriodDays: 30, AdsFree: true, Description: "每 30 天 2,000 AI 点数，去广告"},
		{Code: "monthly-pro", Name: "沉浸月卡", Kind: "SUBSCRIPTION", AmountCents: 3990, CreditAllowance: 4800, PeriodDays: 30, AdsFree: true, Description: "每 30 天 4,800 AI 点数，去广告"},
		{Code: "lifetime", Name: "永久会员", Kind: "LIFETIME", AmountCents: 19900, CreditAllowance: 1200, PeriodDays: 30, AdsFree: true, Description: "永久去广告；每 30 天重置 1,200 AI 点数"},
	}
}

func findBillingProduct(code string) (domain.BillingProduct, bool) {
	for _, product := range billingProducts() {
		if product.Code == strings.TrimSpace(code) {
			return product, true
		}
	}
	return domain.BillingProduct{}, false
}

func (s *Service) BillingProducts() []domain.BillingProduct {
	if !s.SubscriptionFeaturesEnabled() {
		return []domain.BillingProduct{}
	}
	return billingProducts()
}

func (s *Service) CommercialFeaturesEnabled() bool {
	return s != nil && s.billing != nil && !s.billing.options.OpenSourceEdition
}

func (s *Service) MiniProgramFeaturesEnabled() bool {
	return s.CommercialFeaturesEnabled() && s.billing.options.MiniProgramEdition
}

func (s *Service) SubscriptionFeaturesEnabled() bool {
	return s.CommercialFeaturesEnabled() && !s.billing.options.MiniProgramEdition
}

func (s *Service) AICreditCosts() []domain.AICreditCost {
	if !s.CommercialFeaturesEnabled() || s.MiniProgramFeaturesEnabled() {
		return []domain.AICreditCost{}
	}
	return aiCreditCosts()
}

func (s *Service) AdPlacements(userID string) ([]domain.AdPlacement, error) {
	if !s.CommercialFeaturesEnabled() {
		return []domain.AdPlacement{}, nil
	}
	status, err := s.billing.status(userID)
	if err != nil || strings.EqualFold(s.billing.options.AdProvider, "NONE") {
		return []domain.AdPlacement{}, err
	}
	if s.MiniProgramFeaturesEnabled() {
		uses, countErr := s.billing.store.CountMiniProgramUses(userID, s.billing.now())
		if countErr != nil {
			return []domain.AdPlacement{}, countErr
		}
		if uses <= s.billing.options.MiniAdFreeDailyUses {
			return []domain.AdPlacement{}, nil
		}
	} else if status.AdsFree {
		return []domain.AdPlacement{}, nil
	}
	provider := strings.TrimSpace(s.billing.options.AdProvider)
	if provider == "" {
		provider = "MOCK"
	}
	return []domain.AdPlacement{
		{Placement: "COURSES", Provider: provider, ScriptURL: s.billing.options.AdScriptURL, SlotID: s.billing.options.AdCourseSlot},
		{Placement: "LIBRARY", Provider: provider, ScriptURL: s.billing.options.AdScriptURL, SlotID: s.billing.options.AdLibrarySlot},
		{Placement: "RESULT", Provider: provider, ScriptURL: s.billing.options.AdScriptURL, SlotID: s.billing.options.AdResultSlot},
	}, nil
}

func (s *Service) BillingStatus(userID string) (domain.BillingStatus, error) {
	if s.billing == nil {
		return domain.BillingStatus{ProductCode: "free", ProductName: "免费学习者", CreditAllowance: 20, CreditBalance: 20}, nil
	}
	return s.billing.status(userID)
}

func (s *Service) CreatePaymentOrder(userID string, productCode string, channel string) (domain.PaymentOrder, error) {
	if s.billing == nil {
		return domain.PaymentOrder{}, errors.New("billing storage is unavailable")
	}
	return s.billing.createOrder(userID, productCode, channel)
}

func (s *Service) PaymentOrder(userID string, orderID string) (domain.PaymentOrder, error) {
	if s.billing == nil {
		return domain.PaymentOrder{}, errors.New("billing storage is unavailable")
	}
	return s.billing.store.GetPaymentOrder(strings.TrimSpace(orderID), userID)
}

func (s *Service) HandleEpayNotification(values url.Values) error {
	if s.billing == nil {
		return errors.New("billing storage is unavailable")
	}
	return s.billing.handleNotification(values)
}

func (s *Service) ConsumeAIConfidence(userID string, activity string, sourceID string, amount int) error {
	if !s.CommercialFeaturesEnabled() {
		return nil
	}
	if s.MiniProgramFeaturesEnabled() {
		uses, err := s.billing.store.RecordMiniProgramUse(userID, activity, sourceID, s.billing.now())
		if err != nil {
			return err
		}
		if uses > s.billing.options.MiniProgramDailyAIUses {
			_ = s.billing.store.RefundMiniProgramUse(userID, activity, sourceID)
			return fmt.Errorf("产品处于内测：每个账号每天最多可发起 %d 次 AI 使用，请明天再来学习。", s.billing.options.MiniProgramDailyAIUses)
		}
		return nil
	}
	if !s.billing.options.Enabled {
		return nil
	}
	_, err := s.billing.consume(userID, activity, sourceID, amount)
	return err
}

func (s *Service) RefundAIConfidence(userID string, activity string, sourceID string, amount int) {
	if !s.CommercialFeaturesEnabled() {
		return
	}
	if s.MiniProgramFeaturesEnabled() {
		_ = s.billing.store.RefundMiniProgramUse(userID, activity, sourceID)
		return
	}
	if !s.billing.options.Enabled {
		return
	}
	_ = s.billing.refund(userID, activity, sourceID, amount)
}

func (b *billingService) status(userID string) (domain.BillingStatus, error) {
	status, err := b.store.GetBillingStatus(userID, b.now(), b.options.FreeDailyCredits)
	if err != nil {
		return domain.BillingStatus{}, err
	}
	if status.ProductCode == "" {
		status.ProductCode = "free"
		status.ProductName = "免费学习者"
		status.CreditAllowance = b.options.FreeDailyCredits
	}
	return status, nil
}

func (b *billingService) createOrder(userID string, productCode string, channel string) (domain.PaymentOrder, error) {
	if b.options.MiniProgramEdition {
		return domain.PaymentOrder{}, errors.New("小程序版不提供付费服务，仅通过广告支持运营")
	}
	if !b.options.Enabled {
		return domain.PaymentOrder{}, errors.New("payment is not enabled")
	}
	if strings.TrimSpace(b.options.EpayGatewayURL) == "" || strings.TrimSpace(b.options.EpayMerchantID) == "" || strings.TrimSpace(b.options.EpayKey) == "" || strings.TrimSpace(b.options.EpayNotifyURL) == "" {
		return domain.PaymentOrder{}, errors.New("易支付配置不完整")
	}
	product, ok := findBillingProduct(productCode)
	if !ok {
		return domain.PaymentOrder{}, errors.New("unknown billing product")
	}
	return b.createOrderForProduct(userID, product, channel)
}

func (b *billingService) createOrderForProduct(userID string, product domain.BillingProduct, channel string) (domain.PaymentOrder, error) {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "" {
		channel = strings.ToLower(strings.TrimSpace(b.options.DefaultChannel))
	}
	if channel != "alipay" && channel != "wxpay" && channel != "qqpay" {
		return domain.PaymentOrder{}, errors.New("unsupported payment channel")
	}
	order := domain.PaymentOrder{ID: uuid.NewString(), UserID: userID, ProductCode: product.Code, AmountCents: product.AmountCents, PaymentChannel: channel, Status: paymentPending, CreatedAt: b.now()}
	created, err := b.store.CreatePaymentOrder(order)
	if err != nil {
		return domain.PaymentOrder{}, err
	}
	created.CheckoutURL = b.checkoutURL(created, product)
	return created, nil
}

func (b *billingService) checkoutURL(order domain.PaymentOrder, product domain.BillingProduct) string {
	params := url.Values{}
	params.Set("pid", b.options.EpayMerchantID)
	params.Set("type", order.PaymentChannel)
	params.Set("out_trade_no", order.ID)
	params.Set("notify_url", b.options.EpayNotifyURL)
	params.Set("return_url", fmt.Sprintf("%s/membership/complete?order=%s", b.public, url.QueryEscape(order.ID)))
	params.Set("name", "LinguaQuest "+product.Name)
	params.Set("money", centsToMoney(order.AmountCents))
	params.Set("sign", epaySignWithMode(params, b.options.EpayKey, b.options.EpaySignatureMode))
	params.Set("sign_type", "MD5")
	separator := "?"
	if strings.Contains(b.options.EpayGatewayURL, "?") {
		separator = "&"
	}
	return b.options.EpayGatewayURL + separator + params.Encode()
}

func (b *billingService) handleNotification(values url.Values) error {
	if !b.options.Enabled {
		return errors.New("payment is not enabled")
	}
	if values.Get("pid") != b.options.EpayMerchantID || !strings.EqualFold(values.Get("sign"), epaySignWithMode(values, b.options.EpayKey, b.options.EpaySignatureMode)) {
		return errors.New("invalid payment signature")
	}
	if !strings.EqualFold(strings.TrimSpace(values.Get("trade_status")), "TRADE_SUCCESS") {
		return errors.New("payment is not successful")
	}
	orderID := strings.TrimSpace(values.Get("out_trade_no"))
	if orderID == "" {
		return errors.New("missing order number")
	}
	order, err := b.store.GetPaymentOrder(orderID, "")
	if err != nil {
		return err
	}
	if order.Status == paymentPaid {
		return nil
	}
	money, err := moneyToCents(values.Get("money"))
	if err != nil || money != order.AmountCents {
		return errors.New("payment amount mismatch")
	}
	product, ok := findBillingProduct(order.ProductCode)
	if !ok || product.AmountCents != order.AmountCents {
		return errors.New("payment product mismatch")
	}
	_, err = b.store.MarkPaymentOrderPaid(order.ID, strings.TrimSpace(values.Get("trade_no")), product, b.now())
	return err
}

func (b *billingService) consume(userID string, activity string, sourceID string, amount int) (domain.BillingStatus, error) {
	if amount <= 0 {
		return b.status(userID)
	}
	return b.store.ConsumeCredits(userID, activity, sourceID, amount, b.now(), b.options.FreeDailyCredits)
}

func (b *billingService) refund(userID string, activity string, sourceID string, amount int) error {
	return b.store.RefundCredits(userID, activity, sourceID, amount, b.now())
}

func (b *billingService) now() time.Time {
	return time.Now().In(b.location)
}

func epaySign(values url.Values, key string) string {
	return epaySignWithMode(values, key, "RAW_KEY")
}

// epaySignWithMode supports the two MD5 conventions used by EasyPay gateways.
// RAW_KEY is the standard convention: sorted parameters followed directly by the key.
// KEY_VALUE is retained only for legacy gateways that sign an additional key= parameter.
func epaySignWithMode(values url.Values, key string, mode string) string {
	keys := make([]string, 0, len(values))
	for name := range values {
		if name == "sign" || name == "sign_type" || values.Get(name) == "" {
			continue
		}
		keys = append(keys, name)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, name := range keys {
		parts = append(parts, name+"="+values.Get(name))
	}
	payload := strings.Join(parts, "&")
	if strings.EqualFold(strings.TrimSpace(mode), "KEY_VALUE") {
		payload += "&key=" + key
	} else {
		payload += key
	}
	digest := md5.Sum([]byte(payload))
	return hex.EncodeToString(digest[:])
}

func centsToMoney(cents int) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func moneyToCents(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") {
		return 0, errors.New("invalid money")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, errors.New("invalid money")
	}
	whole, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	decimal := 0
	if len(parts) == 2 {
		fraction := parts[1] + "00"
		if len(parts[1]) > 2 {
			return 0, errors.New("invalid money")
		}
		decimal, err = strconv.Atoi(fraction[:2])
		if err != nil {
			return 0, err
		}
	}
	return whole*100 + decimal, nil
}
