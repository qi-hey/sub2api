<template>
  <section ref="sceneHost" class="dynamic-home">
    <canvas ref="sceneCanvas" class="scene-canvas" aria-hidden="true"></canvas>

    <div class="grid-overlay" aria-hidden="true"></div>
    <div class="page-shell">
      <header class="nav-bar">
        <router-link to="/" class="brand" :aria-label="`${siteName} ${t('home.dynamic.home')}`">
          <span v-if="siteLogo" class="brand-logo">
            <img :src="siteLogo" alt="" />
          </span>
          <span v-else class="brand-mark" aria-hidden="true"></span>
          <span class="brand-name">{{ siteName }}</span>
        </router-link>

        <nav class="nav-links" :aria-label="t('home.dynamic.mainNavigation')">
          <a href="#routing-core">{{ t('home.dynamic.capabilities') }}</a>
          <a href="#connected-models">{{ t('home.dynamic.models') }}</a>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza">
            {{ t('nav.modelPlaza') }}
          </router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">
            {{ t('home.docs') }}
          </a>
        </nav>

        <div class="nav-actions">
          <LocaleSwitcher class="locale-switcher" />
          <button
            type="button"
            class="icon-action"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="$emit('toggleTheme')"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <router-link :to="entryPath" class="console-link">
            <span>{{ isAuthenticated ? t('home.dashboard') : t('home.login') }}</span>
            <Icon name="externalLink" size="sm" :stroke-width="1.8" />
          </router-link>
        </div>
      </header>

      <main class="hero">
        <div class="hero-copy">
          <p class="kicker"><span class="kicker-line"></span>{{ kicker }}</p>
          <h1>{{ siteName }}</h1>
          <p class="hero-lead">
            {{ t('home.dynamic.leadPrefix') }}<strong>{{ t('home.dynamic.leadAccent') }}</strong>
          </p>
          <p class="description">{{ t('home.dynamic.description') }}</p>
          <div class="actions">
            <router-link :to="entryPath" class="primary-action">
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.dynamic.startFree') }}
              <Icon name="arrowRight" size="md" :stroke-width="1.8" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="secondary-action"
            >
              {{ t('home.viewDocs') }}
              <Icon name="externalLink" size="md" :stroke-width="1.8" />
            </a>
          </div>
        </div>
      </main>

      <div class="scene-label label-input">{{ t('home.dynamic.input') }}</div>
      <div id="routing-core" class="scene-label label-core">{{ t('home.dynamic.core') }}</div>
      <div class="scene-label label-output">{{ t('home.dynamic.output') }}</div>

      <div id="connected-models" class="model-rail" :aria-label="t('home.dynamic.connectedModels')">
        <strong>{{ t('home.dynamic.connectedModels') }}</strong>
        <span class="rail-pulse" aria-hidden="true"></span>
        <span v-for="model in models" :key="model" class="model-name">{{ model }}</span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { createRoutingScene, type RoutingSceneController } from '@/features/home/createRoutingScene'

const props = defineProps<{
  siteName: string
  siteLogo: string
  siteSubtitle: string
  docUrl: string
  showModelPlazaEntry: boolean
  isAuthenticated: boolean
  dashboardPath: string
  isDark: boolean
}>()

defineEmits<{
  toggleTheme: []
}>()

const { t } = useI18n()
const sceneHost = ref<HTMLElement | null>(null)
const sceneCanvas = ref<HTMLCanvasElement | null>(null)
const models = ['OpenAI', 'Claude', 'Gemini', 'Grok']
const entryPath = computed(() => (props.isAuthenticated ? props.dashboardPath : '/login'))
const kicker = computed(() => props.siteSubtitle.trim() || t('home.dynamic.kicker'))
let sceneController: RoutingSceneController | null = null

onMounted(() => {
  if (!sceneHost.value || !sceneCanvas.value) return
  try {
    sceneController = createRoutingScene(sceneHost.value, sceneCanvas.value)
  } catch (error) {
    console.error('[home] Failed to initialize routing scene:', error)
  }
})

onBeforeUnmount(() => {
  sceneController?.dispose()
  sceneController = null
})
</script>

<style scoped>
.dynamic-home {
  position: relative;
  min-width: 320px;
  height: 100svh;
  min-height: 680px;
  overflow: hidden;
  color: #f2f6f5;
  background: #070b0d;
  font-family: Inter, 'SF Pro Display', 'Segoe UI', 'Microsoft YaHei', system-ui, sans-serif;
}

.scene-canvas {
  position: absolute;
  inset: 0;
  z-index: 0;
  display: block;
  width: 100%;
  height: 100%;
}

.grid-overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(147, 255, 235, 0.025) 1px, transparent 1px),
    linear-gradient(90deg, rgba(147, 255, 235, 0.025) 1px, transparent 1px);
  background-size: 76px 76px;
  mask-image: linear-gradient(90deg, transparent 0%, transparent 28%, #000 68%, #000 100%);
}

.page-shell {
  position: relative;
  z-index: 3;
  width: min(calc(100% - 88px), 1420px);
  height: 100%;
  margin: 0 auto;
}

.nav-bar {
  position: relative;
  display: flex;
  height: 92px;
  align-items: center;
  border-bottom: 1px solid rgba(218, 237, 233, 0.1);
  animation: reveal-down 700ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.brand {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 13px;
  color: #f4f8f7;
  font-size: 20px;
  font-weight: 760;
  text-decoration: none;
}

.brand-name {
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.brand-logo {
  display: grid;
  width: 38px;
  height: 38px;
  flex: none;
  place-items: center;
  overflow: hidden;
  border: 1px solid rgba(218, 237, 233, 0.22);
  background: rgba(255, 255, 255, 0.04);
}

.brand-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.brand-mark {
  position: relative;
  width: 36px;
  height: 36px;
  flex: none;
  border: 1px solid rgba(218, 237, 233, 0.3);
  transform: rotate(45deg);
}

.brand-mark::before,
.brand-mark::after {
  position: absolute;
  inset: 7px;
  border: 2px solid #36d6bd;
  content: '';
  transition: transform 300ms ease;
}

.brand-mark::after {
  inset: 12px;
  border-color: #f0b956;
}

.brand:hover .brand-mark::before {
  transform: rotate(18deg);
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 30px;
  margin-left: auto;
}

.nav-links a {
  position: relative;
  padding: 11px 0;
  color: #91a19f;
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  transition: color 180ms ease;
}

.nav-links a::after {
  position: absolute;
  right: 0;
  bottom: 4px;
  left: 0;
  height: 1px;
  background: #55e0ca;
  content: '';
  transform: scaleX(0);
  transform-origin: right;
  transition: transform 220ms ease;
}

.nav-links a:hover {
  color: #f4f8f7;
}

.nav-links a:hover::after {
  transform: scaleX(1);
  transform-origin: left;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 28px;
}

.icon-action {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 1px solid rgba(218, 237, 233, 0.15);
  color: #bac9c7;
  background: rgba(218, 237, 233, 0.025);
  transition: border-color 180ms ease, color 180ms ease, background 180ms ease;
}

.icon-action:hover {
  border-color: rgba(85, 224, 202, 0.55);
  color: #55e0ca;
  background: rgba(85, 224, 202, 0.07);
}

.console-link {
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  gap: 10px;
  padding: 0 17px;
  border: 1px solid rgba(218, 237, 233, 0.24);
  color: #f4f8f7;
  font-size: 14px;
  font-weight: 700;
  text-decoration: none;
  transition: border-color 180ms ease, background 180ms ease, transform 180ms ease;
}

.console-link:hover {
  border-color: rgba(85, 224, 202, 0.65);
  background: rgba(85, 224, 202, 0.07);
  transform: translateY(-1px);
}

.hero {
  display: flex;
  height: calc(100% - 92px);
  align-items: center;
  padding-bottom: 82px;
}

.hero-copy {
  width: 46%;
  max-width: 650px;
  transform: translateY(-4px);
}

.kicker {
  display: flex;
  align-items: center;
  gap: 13px;
  margin: 0 0 24px;
  color: #55e0ca;
  font-size: 12px;
  font-weight: 750;
  animation: reveal-up 700ms 120ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.kicker-line {
  position: relative;
  width: 44px;
  height: 1px;
  overflow: hidden;
  background: rgba(85, 224, 202, 0.28);
}

.kicker-line::after {
  position: absolute;
  top: 0;
  left: -55%;
  width: 55%;
  height: 1px;
  background: #55e0ca;
  content: '';
  animation: scan-line 2.4s ease-in-out infinite;
}

h1 {
  max-width: 100%;
  margin: 0;
  overflow-wrap: anywhere;
  color: #f4f8f7;
  font-size: 86px;
  line-height: 0.94;
  font-weight: 790;
  letter-spacing: 0;
  animation: reveal-up 820ms 180ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.hero-lead {
  max-width: 620px;
  margin: 28px 0 0;
  color: #dfe8e6;
  font-size: 30px;
  line-height: 1.36;
  font-weight: 620;
  animation: reveal-up 820ms 250ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.hero-lead strong {
  color: #55e0ca;
  font-weight: 700;
}

.description {
  max-width: 555px;
  margin: 22px 0 34px;
  color: #8fa09e;
  font-size: 16px;
  line-height: 1.85;
  animation: reveal-up 820ms 320ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.actions {
  display: flex;
  align-items: center;
  gap: 13px;
  animation: reveal-up 820ms 390ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.primary-action,
.secondary-action {
  display: inline-flex;
  min-height: 52px;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 0 23px;
  font-size: 15px;
  font-weight: 750;
  text-decoration: none;
  transition: transform 180ms ease, border-color 180ms ease, background 180ms ease, color 180ms ease;
}

.primary-action {
  color: #08100f;
  background: #eef5f3;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.28);
}

.primary-action:hover {
  color: #07100e;
  background: #67ead5;
  transform: translateY(-2px);
}

.secondary-action {
  border: 1px solid rgba(218, 237, 233, 0.2);
  color: #dfe8e6;
  background: rgba(218, 237, 233, 0.025);
}

.secondary-action:hover {
  border-color: rgba(240, 185, 86, 0.55);
  color: #f4c873;
  background: rgba(240, 185, 86, 0.055);
  transform: translateY(-2px);
}

.scene-label {
  position: absolute;
  z-index: 4;
  display: flex;
  align-items: center;
  gap: 9px;
  color: rgba(208, 224, 220, 0.68);
  font-size: 11px;
  font-weight: 650;
  pointer-events: none;
  animation: label-breathe 3.2s ease-in-out infinite;
}

.scene-label::before {
  width: 5px;
  height: 5px;
  border: 1px solid #55e0ca;
  content: '';
  transform: rotate(45deg);
}

.label-input {
  top: 45%;
  left: 51%;
}

.label-core {
  top: 31%;
  left: 65%;
  color: rgba(240, 200, 115, 0.76);
  animation-delay: -1s;
}

.label-core::before {
  border-color: #f0b956;
}

.label-output {
  top: 40%;
  right: 1%;
  animation-delay: -2s;
}

.model-rail {
  position: absolute;
  right: 0;
  bottom: 28px;
  left: 0;
  display: flex;
  height: 58px;
  align-items: center;
  gap: 34px;
  overflow: hidden;
  border-top: 1px solid rgba(218, 237, 233, 0.1);
  color: #697a78;
  font-size: 12px;
  white-space: nowrap;
  animation: reveal-up 800ms 520ms cubic-bezier(0.2, 0.7, 0.2, 1) both;
}

.model-rail strong {
  color: #dfe8e6;
  font-weight: 700;
}

.rail-pulse {
  position: relative;
  width: 68px;
  height: 1px;
  overflow: hidden;
  background: rgba(85, 224, 202, 0.16);
}

.rail-pulse::after {
  position: absolute;
  top: 0;
  left: -24px;
  width: 24px;
  height: 1px;
  background: #55e0ca;
  box-shadow: 0 0 12px rgba(85, 224, 202, 0.85);
  content: '';
  animation: rail-flow 1.7s linear infinite;
}

.model-name {
  position: relative;
  color: #849492;
  font-weight: 620;
}

.model-name::after {
  position: absolute;
  top: 50%;
  right: -18px;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: rgba(85, 224, 202, 0.42);
  content: '';
}

@keyframes reveal-up {
  from { opacity: 0; transform: translateY(22px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes reveal-down {
  from { opacity: 0; transform: translateY(-18px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes scan-line {
  0%, 15% { transform: translateX(0); }
  80%, 100% { transform: translateX(180%); }
}

@keyframes rail-flow {
  to { transform: translateX(96px); }
}

@keyframes label-breathe {
  0%, 100% { opacity: 0.48; }
  50% { opacity: 1; }
}

@media (max-width: 1100px) {
  .nav-links { gap: 18px; }
  .nav-actions { margin-left: 18px; }
  .hero-copy { width: 49%; }
  h1 { font-size: 72px; }
}

@media (max-width: 900px) {
  .dynamic-home { min-height: 720px; }
  .grid-overlay {
    background-size: 54px 54px;
    mask-image: linear-gradient(180deg, transparent 0%, transparent 35%, #000 75%, #000 100%);
  }
  .page-shell { width: min(calc(100% - 38px), 720px); }
  .nav-bar { height: 74px; }
  .nav-links { display: none; }
  .nav-actions { margin-left: auto; }
  .hero {
    height: calc(100% - 74px);
    align-items: flex-start;
    padding-top: 54px;
    padding-bottom: 265px;
  }
  .hero-copy { width: 100%; max-width: 520px; }
  .kicker { margin-bottom: 18px; }
  h1 { font-size: 58px; }
  .hero-lead { max-width: 420px; margin-top: 20px; font-size: 23px; }
  .description { max-width: 470px; margin: 15px 0 24px; font-size: 14px; line-height: 1.7; }
  .primary-action, .secondary-action { min-height: 48px; padding: 0 18px; font-size: 14px; }
  .scene-label { display: none; }
  .model-rail { bottom: 16px; gap: 18px; }
  .rail-pulse { width: 34px; }
}

@media (max-width: 640px) {
  .locale-switcher { display: none; }
  .brand-name { max-width: 150px; }
}

@media (max-width: 430px) {
  .brand { font-size: 17px; }
  .brand-mark, .brand-logo { width: 31px; height: 31px; }
  .console-link { padding: 0 12px; }
  .console-link span { display: none; }
  .hero { padding-top: 42px; padding-bottom: 250px; }
  h1 { font-size: 49px; }
  .hero-lead { font-size: 21px; }
  .description { max-width: 350px; }
  .model-rail { gap: 14px; font-size: 11px; }
}

@media (max-height: 720px) and (min-width: 901px) {
  .dynamic-home { min-height: 600px; }
  .nav-bar { height: 74px; }
  .hero { height: calc(100% - 74px); padding-bottom: 52px; }
  .kicker { margin-bottom: 16px; }
  h1 { font-size: 66px; }
  .hero-lead { margin-top: 20px; font-size: 25px; }
  .description { margin: 16px 0 24px; line-height: 1.65; }
  .model-rail { bottom: 12px; height: 48px; }
}

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    scroll-behavior: auto !important;
    animation-duration: 1ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 1ms !important;
  }
}
</style>
