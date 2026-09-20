import { AboutProject } from './AboutProject'

/**
 * Shown when no tab is open — i.e. nothing is connected yet.
 *
 * This is the project's own page: what it is built with, where the source and
 * releases live, who maintains it. Starting a session happens on the ribbon
 * (*Connection* / *Open*) or from the connection tree, so the pane stays a
 * read-only overview.
 */
export function WelcomePane() {
  return (
    <div className="dm-welcome">
      <div className="dm-welcome-inner">
        <AboutProject />
      </div>
    </div>
  )
}
