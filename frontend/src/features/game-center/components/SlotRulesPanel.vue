<template>
  <aside class="slot-rules" data-testid="slot-rules" :aria-labelledby="titleId">
    <header class="slot-rules__header">
      <div>
        <span>{{ t('gameCenter.slot.rules.eyebrow') }}</span>
        <h2 :id="titleId">{{ t('gameCenter.slot.rules.title') }}</h2>
      </div>
      <strong>{{ t('gameCenter.slot.ways') }}</strong>
    </header>

    <section class="slot-rules__section" :aria-labelledby="paytableTitleId">
      <div class="slot-rules__section-title">
        <h3 :id="paytableTitleId">{{ t('gameCenter.slot.rules.paytable') }}</h3>
        <span>{{ t('gameCenter.slot.rules.betScore') }}</span>
      </div>

      <div class="slot-rules__paytable">
        <div v-for="row in linePaytable" :key="row.symbol" class="slot-rules__payrow">
          <div class="slot-rules__symbol">
            <span class="slot-symbol" :class="`slot-symbol--${row.symbol.toLowerCase()}`" aria-hidden="true">
              {{ symbolGlyphs[row.symbol] }}
            </span>
            <strong>{{ t(`gameCenter.slot.symbols.${row.symbol}`) }}</strong>
          </div>
          <div class="slot-rules__pays">
            <span v-for="pay in row.pays" :key="pay.count">
              {{ pay.count }}{{ t('gameCenter.slot.rules.matchSuffix') }}
              {{ t('gameCenter.slot.rules.score', { score: pay.score }) }}
            </span>
          </div>
        </div>

        <div class="slot-rules__payrow slot-rules__payrow--special">
          <div class="slot-rules__symbol">
            <span class="slot-symbol slot-symbol--wild" aria-hidden="true">W</span>
            <strong>{{ t('gameCenter.slot.symbols.WILD') }}</strong>
          </div>
          <div class="slot-rules__pays">
            <span>{{ t('gameCenter.slot.rules.wildSubstitute') }}</span>
          </div>
        </div>

        <div class="slot-rules__payrow slot-rules__payrow--special">
          <div class="slot-rules__symbol">
            <span class="slot-symbol slot-symbol--scatter" aria-hidden="true">★</span>
            <strong>{{ t('gameCenter.slot.symbols.SCATTER') }}</strong>
          </div>
          <div class="slot-rules__pays">
            <span v-for="pay in scatterAwards" :key="pay.count">
              {{ pay.count }}{{ t('gameCenter.slot.rules.matchSuffix') }}
              {{ t('gameCenter.slot.rules.score', { score: pay.score }) }}
              + {{ pay.spins }}{{ t('gameCenter.slot.rules.freeSpinSuffix') }}
            </span>
          </div>
        </div>

        <div class="slot-rules__payrow slot-rules__payrow--special">
          <div class="slot-rules__symbol">
            <span class="slot-symbol slot-symbol--bonus" aria-hidden="true">B</span>
            <strong>{{ t('gameCenter.slot.symbols.BONUS') }}</strong>
          </div>
          <div class="slot-rules__pays">
            <span>{{ t('gameCenter.slot.rules.bonusTrigger') }}</span>
          </div>
        </div>
      </div>

      <p class="slot-rules__note">{{ t('gameCenter.slot.rules.lineNote') }}</p>
      <p class="slot-rules__note">{{ t('gameCenter.slot.rules.specialNote') }}</p>
    </section>

    <section class="slot-rules__section slot-rules__section--ways" :aria-labelledby="waysTitleId">
      <div class="slot-rules__section-title">
        <h3 :id="waysTitleId">{{ t('gameCenter.slot.rules.waysTitle') }}</h3>
        <span>{{ t('gameCenter.slot.rules.leftToRight') }}</span>
      </div>
      <div class="slot-rules__ways">
        <strong>3 × 3 × 3 × 3 × 3 = 243</strong>
        <p>{{ t('gameCenter.slot.rules.waysDetail') }}</p>
      </div>
    </section>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import {
  SLOT_LINE_PAYTABLE,
  SLOT_SCATTER_FREE_SPINS,
  SLOT_SCATTER_PAYTABLE,
} from '../slotSymbols'

type LinePaySymbol = keyof typeof SLOT_LINE_PAYTABLE

const { t } = useI18n()
const titleId = 'slot-rules-title'
const paytableTitleId = 'slot-rules-paytable-title'
const waysTitleId = 'slot-rules-ways-title'

const symbolGlyphs: Record<LinePaySymbol, string> = {
  JP: '7',
  DR: '龙',
  IG: '元',
  JA: '玉',
  BE: '铃',
}

const linePaytable = (Object.entries(SLOT_LINE_PAYTABLE) as Array<[
  LinePaySymbol,
  Record<number, number>,
]>).map(([symbol, pays]) => ({
  symbol,
  pays: Object.entries(pays).map(([count, score]) => ({
    count: Number(count),
    score,
  })),
}))

const scatterAwards = Object.entries(SLOT_SCATTER_FREE_SPINS).map(([count, spins]) => ({
  count: Number(count),
  score: SLOT_SCATTER_PAYTABLE[Number(count) as keyof typeof SLOT_SCATTER_PAYTABLE],
  spins,
}))

</script>

<style scoped>
.slot-rules {
  width: 100%;
  max-height: 600px;
  overflow-y: auto;
  border: 1px solid rgba(251, 191, 36, 0.55);
  border-radius: 8px;
  background: #180d2c;
  color: #fff7d6;
  box-shadow: 0 18px 42px rgba(38, 10, 64, 0.28);
  scrollbar-color: #a855f7 #180d2c;
}

.slot-rules__header {
  position: sticky;
  z-index: 2;
  top: 0;
  display: flex;
  min-height: 72px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 13px 16px;
  border-bottom: 2px solid #d97706;
  background: #6b1526;
}

.slot-rules__header span,
.slot-rules__section-title span {
  display: block;
  color: #fcd34d;
  font-size: 10px;
  font-weight: 800;
  line-height: 1.2;
  text-transform: uppercase;
}

.slot-rules__header h2,
.slot-rules__section-title h3 {
  margin: 0;
  letter-spacing: 0;
}

.slot-rules__header h2 {
  margin-top: 2px;
  color: #fff7c2;
  font-size: 21px;
  line-height: 1.15;
}

.slot-rules__header > strong {
  flex: none;
  color: #fff1a8;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
}

.slot-rules__section {
  padding: 15px 16px;
  border-bottom: 1px solid rgba(251, 191, 36, 0.2);
}

.slot-rules__section:last-child {
  border-bottom: 0;
}

.slot-rules__section-title {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.slot-rules__section-title h3 {
  color: #fff7d6;
  font-size: 14px;
  line-height: 1.3;
}

.slot-rules__section-title span {
  color: #c4b5fd;
  text-align: right;
}

.slot-rules__paytable {
  display: grid;
}

.slot-rules__payrow {
  display: grid;
  min-height: 44px;
  grid-template-columns: minmax(100px, 0.8fr) minmax(0, 1.2fr);
  align-items: center;
  gap: 8px;
  padding: 5px 0;
  border-bottom: 1px solid rgba(196, 181, 253, 0.12);
}

.slot-rules__payrow:last-child {
  border-bottom: 0;
}

.slot-rules__payrow--special {
  background: rgba(107, 21, 38, 0.2);
}

.slot-rules__symbol {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.slot-rules__symbol strong {
  overflow: hidden;
  font-size: 12px;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slot-symbol {
  display: grid;
  width: 30px;
  height: 30px;
  flex: none;
  place-items: center;
  border: 2px solid #ffe48a;
  border-radius: 6px;
  background: #7f1d1d;
  color: white;
  font-family: "Noto Sans SC", ui-sans-serif, sans-serif;
  font-size: 16px;
  font-weight: 900;
  line-height: 1;
  box-shadow: inset 0 0 0 2px rgba(255, 255, 255, 0.18);
}

.slot-symbol--jp { background: #dc2626; color: #fef08a; font-size: 22px; }
.slot-symbol--dr { background: #b91c1c; color: #fde68a; }
.slot-symbol--ig { background: #f59e0b; color: #6b2105; }
.slot-symbol--ja { background: #10b981; color: #ecfdf5; }
.slot-symbol--be { background: #eab308; color: #713f12; }
.slot-symbol--wild { background: #2563eb; color: #fff; }
.slot-symbol--scatter { background: #db2777; color: #fef08a; }
.slot-symbol--bonus { background: #7c3aed; color: #fef08a; }

.slot-rules__pays {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 2px 8px;
  color: #e9d5ff;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.35;
  text-align: right;
}

.slot-rules__note {
  margin: 9px 0 0;
  color: #c4b5fd;
  font-size: 10px;
  line-height: 1.45;
}

.slot-rules__ways {
  padding: 12px;
  border: 1px solid rgba(251, 191, 36, 0.28);
  border-radius: 6px;
  background: rgba(49, 20, 74, 0.82);
  text-align: center;
}

.slot-rules__ways strong {
  display: block;
  color: #fcd34d;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 18px;
}

.slot-rules__ways p {
  margin: 7px 0 0;
  color: #e9d5ff;
  font-size: 11px;
  line-height: 1.55;
}

@media (max-width: 899px) {
  .slot-rules {
    max-height: none;
    overflow: visible;
  }

  .slot-rules__header {
    position: static;
  }

}

@media (max-width: 519px) {
  .slot-rules__header,
  .slot-rules__section {
    padding-right: 12px;
    padding-left: 12px;
  }

  .slot-rules__payrow {
    grid-template-columns: minmax(90px, 0.75fr) minmax(0, 1.25fr);
  }

}
</style>
