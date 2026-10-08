import React from 'react'

// Wraps a timer row's mute / hide / remove buttons. By default the cluster is
// hidden and floats over the row's right edge only while the row is hovered
// (see .timer-row-controls in index.css), so it costs no horizontal space.
// alwaysShow (Settings → Spell Timers) restores the inline always-visible
// layout. The row root must carry the "timer-row" class.
export default function TimerRowControls({
  alwaysShow,
  children,
}: {
  alwaysShow: boolean
  children: React.ReactNode
}): React.ReactElement {
  if (alwaysShow) return <>{children}</>
  return <span className="timer-row-controls">{children}</span>
}
