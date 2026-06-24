<script setup lang="ts">
// A static, hand-styled rendering of the `wf` dashboard ledger using the real
// glyphs, columns, and colours of the lipgloss TUI: a project header, the ◆ base
// checkout row, worktrees as tree children, a live agent-status icon, and the
// trailing ▣ tmux-open marker. It is decorative — see docs/capture/ for the
// procedure that regenerates genuine captures.
//
// The agent icons use the emoji preset (🤖/⏳) so they render in any browser; the
// real dashboard defaults to Nerd Font glyphs (configurable — see the Agent
// Status guide).
//
// Each line is an array of {t: text, c: css-class} segments. Spaces live inside
// the interpolated strings so VitePress's whitespace handling preserves them.
type Seg = { t: string; c?: string }
type Line = { sel?: boolean; segs: Seg[] }

const sp = (n: number): Seg => ({ t: ' '.repeat(n) })

const head: Seg = {
  t: '        branch          state   behind|ahead diff        base',
  c: 't-head',
}

const lines: Line[] = [
  { segs: [{ t: 'WorkFlow — dashboard', c: 't-title' }] },
  { segs: [sp(1)] },
  // selected project header (dark-on-cyan bar)
  { sel: true, segs: [{ t: '❯ acme-api (2)  ~/code/acme-api' }] },
  { segs: [head] },
  // base checkout row: ◆ marker, the root's branch + clean/dirty state
  {
    segs: [
      sp(2),
      { t: '◆', c: 't-base' }, sp(4),
      { t: '○', c: 't-clean' },
      { t: ' development     ' },
      { t: 'clean', c: 't-clean' },
    ],
  },
  // a working agent in this worktree (🤖), its window open (▣)
  {
    segs: [
      sp(2),
      { t: '├─ ', c: 't-dim' },
      { t: '🤖', c: 't-work' }, sp(1),
      { t: '●', c: 't-active' },
      { t: ' feature/login   ' },
      { t: 'active', c: 't-active' }, sp(2),
      { t: '↓0|↑3', c: 't-dim' }, sp(6),
      { t: '+84', c: 't-add' }, sp(1), { t: '-12', c: 't-del' }, sp(1),
      { t: '*', c: 't-star' }, sp(2),
      { t: 'development', c: 't-dim' }, sp(2),
      { t: '▣', c: 't-tmux' },
    ],
  },
  {
    segs: [
      sp(2),
      { t: '└─ ', c: 't-dim' }, sp(2),
      { t: '○', c: 't-clean' },
      { t: ' fix/cache-key   ' },
      { t: 'clean', c: 't-clean' }, sp(3),
      { t: '↓1|↑0', c: 't-dim' }, sp(6),
      { t: '+0', c: 't-add' }, sp(1), { t: '-0', c: 't-del' }, sp(5),
      { t: 'development', c: 't-dim' },
    ],
  },
  { segs: [sp(1)] },
  { segs: [sp(2), { t: 'dotfiles (1)  ~/code/dotfiles', c: 't-proj' }] },
  { segs: [head] },
  {
    segs: [
      sp(2),
      { t: '◆', c: 't-base' }, sp(4),
      { t: '○', c: 't-clean' },
      { t: ' main            ' },
      { t: 'clean', c: 't-clean' },
    ],
  },
  // a worktree waiting on input (⏳)
  {
    segs: [
      sp(2),
      { t: '└─ ', c: 't-dim' },
      { t: '⏳', c: 't-wait' }, sp(1),
      { t: '●', c: 't-active' },
      { t: ' tmux-theme      ' },
      { t: 'active', c: 't-active' }, sp(2),
      { t: '↓0|↑1', c: 't-dim' }, sp(6),
      { t: '+20', c: 't-add' }, sp(1), { t: '-4', c: 't-del' }, sp(6),
      { t: 'main', c: 't-dim' },
    ],
  },
  { segs: [sp(1)] },
  {
    segs: [
      { t: '🤖', c: 't-work' }, { t: ' working  ', c: 't-dim' },
      { t: '⏳', c: 't-wait' }, { t: ' waiting  ', c: 't-dim' },
      { t: '●', c: 't-active' }, { t: ' active  ', c: 't-dim' },
      { t: '○', c: 't-clean' }, { t: ' clean  ', c: 't-dim' },
      { t: '◆', c: 't-base' }, { t: ' base  ', c: 't-dim' },
      { t: '▣', c: 't-tmux' }, { t: ' tmux open  ', c: 't-dim' },
      { t: '+added', c: 't-add' }, sp(1), { t: '-removed', c: 't-del' }, sp(2),
      { t: '*', c: 't-star' }, { t: ' uncommitted', c: 't-dim' },
    ],
  },
  {
    segs: [
      { t: '↑/↓ move · enter diff/menu · a add · e edit · o config · t term · c copy · m merge · x rm · r refresh · q quit', c: 't-help' },
    ],
  },
]
</script>

<template>
  <div class="wf-terminal">
    <div class="wf-terminal__bar">
      <span class="wf-terminal__dot wf-terminal__dot--r" />
      <span class="wf-terminal__dot wf-terminal__dot--y" />
      <span class="wf-terminal__dot wf-terminal__dot--g" />
      <span class="wf-terminal__title">wf — dashboard</span>
    </div>
    <div class="wf-terminal__body">
      <div
        v-for="(line, i) in lines"
        :key="i"
        class="wf-terminal__line"
        :class="{ 't-sel': line.sel }"
      ><span v-for="(seg, j) in line.segs" :key="j" :class="seg.c">{{ seg.t }}</span></div>
    </div>
  </div>
</template>
