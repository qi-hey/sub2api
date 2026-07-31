<template>
  <div
    class="game-art"
    :class="[`game-art--${gameId}`, { 'game-art--featured': featured }]"
    :style="{ '--game-accent': accent }"
    aria-hidden="true"
  >
    <div class="game-art__grid"></div>

    <template v-if="gameId === 'snake'">
      <div class="snake snake--1"></div>
      <div class="snake snake--2"></div>
      <div class="snake snake--3"></div>
      <div class="snake snake--4"></div>
      <div class="snake snake--5"></div>
      <div class="snake-food"></div>
    </template>

    <template v-else-if="gameId === 'tetris'">
      <div class="tetris-stack">
        <i v-for="index in 18" :key="index" :class="`tetris-block tetris-block--${index}`"></i>
      </div>
    </template>

    <template v-else-if="gameId === 'breakout'">
      <div class="breakout-bricks">
        <i v-for="index in 15" :key="index"></i>
      </div>
      <div class="breakout-ball"></div>
      <div class="breakout-trail"></div>
      <div class="breakout-paddle"></div>
    </template>

    <template v-else-if="gameId === 'merge2048'">
      <div class="fruit-cabinet">
        <div class="fruit-cabinet__title">欢乐水果机</div>
        <div class="fruit-ring">
          <i v-for="symbol in ['BAR', '77', '星', '瓜', '铃', '芒', '橙', '果']" :key="symbol">{{ symbol }}</i>
        </div>
        <div class="fruit-cabinet__display">8888</div>
      </div>
    </template>

    <template v-else-if="gameId === 'orbital'">
      <div class="orbit orbit--outer"><i></i></div>
      <div class="orbit orbit--inner"><i></i></div>
      <div class="planet"><span></span></div>
      <div class="orbital-bolt orbital-bolt--1"></div>
      <div class="orbital-bolt orbital-bolt--2"></div>
    </template>

    <template v-else-if="gameId === 'hoarder'">
      <div class="hoarder-vault">
        <div class="hoarder-screen">8.8.8</div>
        <div class="hoarder-door"><i></i></div>
        <div class="hoarder-lights"><i></i><i></i><i></i></div>
      </div>
      <div class="hoarder-chip hoarder-chip--1">+</div>
      <div class="hoarder-chip hoarder-chip--2">+</div>
      <div class="hoarder-chip hoarder-chip--3">+</div>
    </template>

    <template v-else>
      <div class="slot-glow"></div>
      <div class="slot-machine">
        <div class="slot-crown"><i></i><span>头奖</span><i></i></div>
        <div class="slot-screen">
          <span class="slot-reel">7</span>
          <span class="slot-reel slot-reel--bar">金条</span>
          <span class="slot-reel">7</span>
        </div>
        <div class="slot-controls"><i></i><i></i><b>旋转</b></div>
        <div class="slot-handle"><i></i><b></b></div>
      </div>
      <div class="slot-stars slot-stars--left">777</div>
      <div class="slot-stars slot-stars--right">777</div>
    </template>

    <div class="game-art__scanline"></div>
  </div>
</template>

<script setup lang="ts">
import type { GameId } from '../types'

withDefaults(defineProps<{
  gameId: GameId
  accent: string
  featured?: boolean
}>(), {
  featured: false,
})
</script>

<style scoped>
.game-art {
  --game-accent: #2dd4bf;
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 10rem;
  overflow: hidden;
  background: #070a12;
  isolation: isolate;
}

.game-art::before {
  position: absolute;
  inset: 0;
  z-index: -1;
  background: radial-gradient(circle at 50% 50%, color-mix(in srgb, var(--game-accent) 21%, transparent), transparent 64%);
  content: '';
}

.game-art__grid {
  position: absolute;
  inset: 0;
  opacity: 0.2;
  background-image:
    linear-gradient(color-mix(in srgb, var(--game-accent) 28%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--game-accent) 28%, transparent) 1px, transparent 1px);
  background-size: 24px 24px;
  transform: perspective(220px) rotateX(55deg) scale(1.35) translateY(29%);
  transform-origin: bottom;
}

.game-art__scanline {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.15;
  background: repeating-linear-gradient(0deg, transparent 0 3px, #000 4px);
}

.snake {
  position: absolute;
  width: 26px;
  height: 26px;
  border: 2px solid color-mix(in srgb, var(--game-accent) 72%, white);
  background: color-mix(in srgb, var(--game-accent) 52%, #06120b);
  box-shadow: 0 0 16px color-mix(in srgb, var(--game-accent) 52%, transparent);
}

.snake--1 { left: 24%; top: 57%; }
.snake--2 { left: calc(24% + 27px); top: 57%; }
.snake--3 { left: calc(24% + 54px); top: 57%; }
.snake--4 { left: calc(24% + 54px); top: calc(57% - 27px); }
.snake--5 { left: calc(24% + 54px); top: calc(57% - 54px); }

.snake--5::before,
.snake--5::after {
  position: absolute;
  top: 6px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #07110a;
  content: '';
}

.snake--5::before { left: 5px; }
.snake--5::after { right: 5px; }

.snake-food {
  position: absolute;
  top: 36%;
  right: 24%;
  width: 20px;
  height: 20px;
  border: 3px solid #fecdd3;
  border-radius: 50%;
  background: #fb7185;
  box-shadow: 0 0 22px #fb7185;
}

.tetris-stack {
  position: absolute;
  left: 50%;
  bottom: 16%;
  display: grid;
  grid-template-columns: repeat(6, 25px);
  gap: 3px;
  transform: translateX(-50%);
}

.tetris-block {
  width: 25px;
  height: 25px;
  border: 2px solid rgba(255, 255, 255, 0.46);
  background: var(--game-accent);
  box-shadow: 0 0 12px color-mix(in srgb, var(--game-accent) 50%, transparent);
}

.tetris-block:nth-child(4n + 1) { background: #f472b6; }
.tetris-block:nth-child(4n + 2) { background: #fbbf24; }
.tetris-block:nth-child(4n + 3) { background: #4ade80; }
.tetris-block--1,
.tetris-block--2,
.tetris-block--6,
.tetris-block--7 { visibility: hidden; }

.breakout-bricks {
  position: absolute;
  top: 19%;
  left: 50%;
  display: grid;
  width: min(74%, 250px);
  grid-template-columns: repeat(5, 1fr);
  gap: 5px;
  transform: translateX(-50%);
}

.breakout-bricks i {
  height: 14px;
  border: 1px solid rgba(255, 255, 255, 0.42);
  background: var(--game-accent);
  box-shadow: 0 0 8px color-mix(in srgb, var(--game-accent) 45%, transparent);
}

.breakout-bricks i:nth-child(3n + 1) { background: #38bdf8; }
.breakout-bricks i:nth-child(3n + 2) { background: #fbbf24; }

.breakout-paddle {
  position: absolute;
  bottom: 17%;
  left: 50%;
  width: 92px;
  height: 11px;
  border-radius: 5px;
  background: #f8fafc;
  box-shadow: 0 0 18px var(--game-accent);
  transform: translateX(-50%);
}

.breakout-ball {
  position: absolute;
  top: 53%;
  left: 59%;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 0 17px #fff;
}

.breakout-trail {
  position: absolute;
  top: 57%;
  left: 56%;
  width: 2px;
  height: 50px;
  background: var(--game-accent);
  opacity: 0.7;
  transform: rotate(28deg);
}

.fruit-cabinet {
  position: absolute;
  top: 12%;
  left: 50%;
  width: min(80%, 280px);
  height: 76%;
  border: 4px solid #e3b341;
  border-radius: 8px;
  background: #74151d;
  box-shadow: 0 0 22px rgba(227, 179, 65, 0.4);
  transform: translateX(-50%);
}

.fruit-cabinet__title {
  padding: 5px;
  color: #fff0a8;
  font-size: 13px;
  font-weight: 800;
  text-align: center;
}

.fruit-ring {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 4px;
  padding: 5px 10px;
}

.fruit-ring i {
  display: grid;
  height: 28px;
  place-items: center;
  border: 2px solid #e3b341;
  border-radius: 3px;
  background: #fef3c7;
  color: #74151d;
  font-size: 10px;
  font-style: normal;
  font-weight: 900;
}

.fruit-cabinet__display {
  width: 46%;
  margin: 7px auto;
  border: 2px solid #d6a83c;
  background: #143a28;
  color: #ffdd57;
  font-family: ui-monospace, monospace;
  font-size: 20px;
  font-weight: 900;
  text-align: center;
}

.planet {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 78px;
  height: 78px;
  border: 3px solid #bae6fd;
  border-radius: 50%;
  background: #075985;
  box-shadow: 0 0 35px var(--game-accent), inset -18px -10px 22px #082f49;
  transform: translate(-50%, -50%);
}

.planet span {
  position: absolute;
  top: 19px;
  left: 15px;
  width: 22px;
  height: 11px;
  border-radius: 50%;
  background: #7dd3fc;
  opacity: 0.72;
  transform: rotate(-18deg);
}

.orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  border: 1px solid color-mix(in srgb, var(--game-accent) 70%, white);
  border-radius: 50%;
  transform: translate(-50%, -50%) rotate(-18deg);
}

.orbit--outer { width: 228px; height: 112px; }
.orbit--inner { width: 170px; height: 86px; transform: translate(-50%, -50%) rotate(46deg); }

.orbit i {
  position: absolute;
  top: 50%;
  right: -8px;
  width: 15px;
  height: 15px;
  border: 2px solid white;
  background: var(--game-accent);
  box-shadow: 0 0 18px var(--game-accent);
}

.orbital-bolt {
  position: absolute;
  width: 3px;
  height: 38px;
  background: #f472b6;
  box-shadow: 0 0 14px #f472b6;
}

.orbital-bolt--1 { top: 22%; left: 25%; transform: rotate(-52deg); }
.orbital-bolt--2 { right: 24%; bottom: 18%; transform: rotate(52deg); }

.hoarder-vault {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 164px;
  height: 126px;
  border: 3px solid color-mix(in srgb, var(--game-accent) 70%, white);
  background: #17210c;
  box-shadow: 0 0 26px color-mix(in srgb, var(--game-accent) 34%, transparent);
  transform: translate(-50%, -50%);
}

.hoarder-screen {
  position: absolute;
  top: 14px;
  left: 17px;
  padding: 4px 7px;
  border: 1px solid var(--game-accent);
  background: #050907;
  color: var(--game-accent);
  font-family: ui-monospace, monospace;
  font-size: 11px;
}

.hoarder-door {
  position: absolute;
  right: 17px;
  bottom: 16px;
  width: 76px;
  height: 76px;
  border: 3px solid #94a3b8;
  border-radius: 50%;
  background: #111827;
}

.hoarder-door::before,
.hoarder-door::after,
.hoarder-door i {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 48px;
  height: 4px;
  background: #94a3b8;
  content: '';
  transform: translate(-50%, -50%);
}

.hoarder-door::after { transform: translate(-50%, -50%) rotate(60deg); }
.hoarder-door i { transform: translate(-50%, -50%) rotate(-60deg); }

.hoarder-lights {
  position: absolute;
  bottom: 17px;
  left: 20px;
  display: grid;
  gap: 9px;
}

.hoarder-lights i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--game-accent);
  box-shadow: 0 0 10px var(--game-accent);
}

.hoarder-chip {
  position: absolute;
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border: 2px dashed #d9f99d;
  border-radius: 50%;
  background: #365314;
  color: #ecfccb;
  font-weight: 900;
}

.hoarder-chip--1 { top: 23%; right: 19%; }
.hoarder-chip--2 { bottom: 17%; left: 15%; }
.hoarder-chip--3 { top: 18%; left: 18%; transform: scale(0.72); }

.slot-glow {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 58%;
  height: 70%;
  border-radius: 50%;
  background: color-mix(in srgb, var(--game-accent) 28%, transparent);
  filter: blur(28px);
  transform: translate(-50%, -50%);
}

.slot-machine {
  position: absolute;
  top: 52%;
  left: 50%;
  width: min(68%, 320px);
  height: 76%;
  min-height: 132px;
  border: 3px solid #fcd34d;
  border-radius: 8px 8px 5px 5px;
  background: #3b1d09;
  box-shadow: 0 0 12px #f59e0b, 0 0 38px rgba(244, 114, 182, 0.3), inset 0 0 20px rgba(251, 191, 36, 0.2);
  transform: translate(-50%, -50%);
}

.slot-crown {
  position: absolute;
  top: -24px;
  left: 50%;
  display: flex;
  width: 74%;
  height: 30px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  border: 2px solid #fcd34d;
  border-radius: 7px 7px 2px 2px;
  background: #7c2d12;
  color: #fef3c7;
  font-family: ui-monospace, monospace;
  font-size: clamp(9px, 2.2vw, 14px);
  font-weight: 900;
  box-shadow: 0 0 14px #f59e0b;
  transform: translateX(-50%);
}

.slot-crown i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #f9a8d4;
  box-shadow: 0 0 8px #f472b6;
}

.slot-screen {
  position: absolute;
  top: 23%;
  left: 8%;
  display: grid;
  width: 84%;
  height: 46%;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 5px;
  padding: 6px;
  border: 3px solid #f59e0b;
  border-radius: 5px;
  background: #0c0a09;
  box-shadow: inset 0 0 14px #000, 0 0 12px rgba(251, 191, 36, 0.5);
}

.slot-reel {
  display: grid;
  min-width: 0;
  place-items: center;
  border: 2px solid #fed7aa;
  border-radius: 4px;
  background: #fff7ed;
  color: #dc2626;
  font-family: ui-monospace, monospace;
  font-size: clamp(24px, 6vw, 44px);
  font-weight: 950;
  text-shadow: 1px 1px 0 #fbbf24;
}

.slot-reel--bar {
  color: #111827;
  font-size: clamp(10px, 2.8vw, 19px);
}

.slot-controls {
  position: absolute;
  right: 9%;
  bottom: 8%;
  display: flex;
  align-items: center;
  gap: 7px;
}

.slot-controls i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #f472b6;
  box-shadow: 0 0 8px #f472b6;
}

.slot-controls i:nth-child(2) {
  background: #22d3ee;
  box-shadow: 0 0 8px #22d3ee;
}

.slot-controls b {
  padding: 3px 8px;
  border: 1px solid #fcd34d;
  border-radius: 4px;
  background: #b45309;
  color: #fef3c7;
  font-family: ui-monospace, monospace;
  font-size: 8px;
}

.slot-handle {
  position: absolute;
  top: 28%;
  right: -26px;
  width: 23px;
  height: 58%;
}

.slot-handle i {
  position: absolute;
  right: 8px;
  bottom: 0;
  width: 5px;
  height: 48px;
  border-radius: 3px;
  background: #fcd34d;
  transform: rotate(8deg);
  transform-origin: bottom;
}

.slot-handle b {
  position: absolute;
  top: -2px;
  right: 1px;
  width: 19px;
  height: 19px;
  border: 2px solid #fecdd3;
  border-radius: 50%;
  background: #e11d48;
  box-shadow: 0 0 13px #fb7185;
}

.slot-stars {
  position: absolute;
  top: 48%;
  color: rgba(251, 191, 36, 0.42);
  font-family: ui-monospace, monospace;
  font-size: 10px;
  font-weight: 900;
  writing-mode: vertical-rl;
}

.slot-stars--left { left: 7%; }
.slot-stars--right { right: 7%; transform: rotate(180deg); }

.game-art--featured .slot-machine {
  width: min(57%, 350px);
}

.game-art--featured .slot-reel {
  font-size: clamp(31px, 5vw, 50px);
}

@media (prefers-reduced-motion: no-preference) {
  .snake-food,
  .breakout-ball,
  .orbit i,
  .slot-crown i,
  .hoarder-lights i {
    animation: art-pulse 2.2s ease-in-out infinite alternate;
  }
}

@keyframes art-pulse {
  from { opacity: 0.65; }
  to { opacity: 1; filter: brightness(1.3); }
}
</style>
