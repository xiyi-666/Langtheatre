import { ArrowLeft, CalendarDays, CheckCircle2, History, Sparkles } from "lucide-react";
import { Link } from "react-router-dom";
import { currentProductVersion, releaseNotes } from "../product";

export function ReleaseNotesPage() {
  return (
    <main className="page release-notes-page">
      <section className="release-notes-shell">
        <header className="release-notes-hero">
          <Link className="release-notes-back" to="/login"><ArrowLeft size={16} /> 返回登录</Link>
          <span className="eyebrow"><History size={15} /> Feature guide</span>
          <div className="release-notes-hero-copy">
            <div>
              <span className="release-notes-version">当前版本 · V{currentProductVersion}</span>
              <h1>版本功能介绍</h1>
              <p>这里集中介绍 LinguaQuest 已上线的学习能力，帮助你快速了解每个版本可以做什么。</p>
            </div>
            <div className="release-notes-seal" aria-hidden><Sparkles size={24} /></div>
          </div>
        </header>

        <section className="release-notes-timeline" aria-label="版本更新记录">
          {releaseNotes.map((release, index) => (
            <article className="release-note-card" key={release.version}>
              <div className="release-note-marker" aria-hidden><span>{index + 1}</span></div>
              <div className="release-note-head">
                <div>
                  <span className="release-notes-version">V{release.version}</span>
                  <h2>{release.title}</h2>
                </div>
                <time dateTime={release.releasedOn}><CalendarDays size={15} /> {release.releasedOn}</time>
              </div>
              <p>{release.summary}</p>
              <ul>
                {release.highlights.map((highlight) => <li key={highlight}><CheckCircle2 size={17} /> <span>{highlight}</span></li>)}
              </ul>
            </article>
          ))}
        </section>

      </section>
    </main>
  );
}
