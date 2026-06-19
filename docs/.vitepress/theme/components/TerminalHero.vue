<script setup lang="ts">
// A static, hand-styled rendering of the `wf` dashboard ledger using the real
// glyphs, columns, and colours. It is decorative — see docs/capture/ for the
// procedure that regenerates genuine captures once the TUI redesign lands.
//
// Each line is an array of {t: text, c: css-class} segments. Spaces live inside
// the interpolated strings so VitePress's whitespace handling preserves them.
type Seg = { t: string; c?: string }
type Line = { sel?: boolean; segs: Seg[] }

const sp = (n: number): Seg => ({ t: ' '.repeat(n) })

const lines: Line[] = [
  { segs: [{ t: 'WorkFlow — dashboard', c: 't-title' }] },
  { segs: [sp(1)] },
  // selected project header (dark-on-cyan bar)
  { sel: true, segs: [{ t: '❯ acme-api (2)  ~/code/acme-api' }] },
  { segs: [{ t: '      branch                 state   behind|ahead diff         base', c: 't-head' }] },
  {
    segs: [
      sp(2),
      { t: '▣', c: 't-tmux' }, sp(1),
      { t: '●', c: 't-active' },
      { t: ' feature/login          ' },
      { t: 'active', c: 't-active' }, sp(2),
      { t: '↓0|↑3', c: 't-dim' }, sp(8),
      { t: '+84', c: 't-add' }, sp(1), { t: '-12', c: 't-del' }, sp(1),
      { t: '*', c: 't-star' }, sp(3),
      { t: 'development', c: 't-dim' },
    ],
  },
  {
    segs: [
      sp(4),
      { t: '○', c: 't-clean' },
      { t: ' fix/cache-key          ' },
      { t: 'clean', c: 't-clean' }, sp(3),
      { t: '↓1|↑0', c: 't-dim' }, sp(8),
      { t: '+0', c: 't-add' }, sp(1), { t: '-0', c: 't-del' }, sp(6),
      { t: 'development', c: 't-dim' },
    ],
  },
  { segs: [sp(1)] },
  { segs: [sp(2), { t: 'dotfiles (1)  ~/code/dotfiles', c: 't-proj' }] },
  { segs: [{ t: '      branch                 state   behind|ahead diff         base', c: 't-head' }] },
  {
    segs: [
      sp(4),
      { t: '●', c: 't-active' },
      { t: ' tmux-theme             ' },
      { t: 'active', c: 't-active' }, sp(2),
      { t: '↓0|↑1', c: 't-dim' }, sp(8),
      { t: '+20', c: 't-add' }, sp(1), { t: '-4', c: 't-del' }, sp(7),
      { t: 'main', c: 't-dim' },
    ],
  },
  { segs: [sp(1)] },
  {
    segs: [
      { t: '●', c: 't-active' }, { t: ' active   ', c: 't-dim' },
      { t: '○', c: 't-clean' }, { t: ' clean   ', c: 't-dim' },
      { t: '▣', c: 't-tmux' }, { t: ' tmux open   ↓behind|↑ahead vs base   ', c: 't-dim' },
      { t: '+added', c: 't-add' }, sp(1), { t: '-removed', c: 't-del' }, sp(3),
      { t: '*', c: 't-star' }, { t: ' uncommitted', c: 't-dim' },
    ],
  },
  {
    segs: [
      { t: '↑/↓ move · enter diff · a add · o edit · t term · c copy · m merge · x rm · r refresh · q quit', c: 't-help' },
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
