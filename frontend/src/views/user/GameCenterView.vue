<template>
  <AppLayout>
    <div class="game-center">
      <section
        class="arcade-hero"
        :style="{ backgroundImage: `url(${arcadeHallUrl})` }"
        aria-labelledby="game-center-title"
      >
        <div class="arcade-hero__shade"></div>
        <div class="arcade-hero__content">
          <span class="arcade-hero__brand">SUB2API</span>
          <h1 id="game-center-title">{{ t('gameCenter.title') }}</h1>
        </div>
        <div class="arcade-hero__counter" :aria-label="`${t('gameCenter.catalogTitle')}: ${games.length}`">
          <span>{{ String(games.length).padStart(2, '0') }}</span>
        </div>
      </section>

      <section class="arcade-catalog" :aria-label="t('gameCenter.catalogTitle')">
        <div class="arcade-catalog__heading">
          <h2>{{ t('gameCenter.catalogTitle') }}</h2>
          <span>{{ String(games.length).padStart(2, '0') }}</span>
        </div>

        <div class="arcade-grid">
          <RouterLink
            v-for="game in games"
            :key="game.id"
            :to="`/games/${game.id}`"
            class="game-card"
            :class="{ 'game-card--featured': game.id === 'lucky' }"
            :style="{ '--game-accent': game.accent, '--game-accent-soft': game.accentSoft }"
            :aria-label="`${t(game.titleKey)} - ${t('gameCenter.actions.play')}`"
            :data-game-id="game.id"
          >
            <div class="game-card__art">
              <GameCardArt
                :game-id="game.id"
                :accent="game.accent"
                :featured="game.id === 'lucky'"
              />
            </div>

            <div class="game-card__body">
              <div class="game-card__copy">
                <h3>{{ t(game.titleKey) }}</h3>
                <p>{{ t(game.subtitleKey) }}</p>
              </div>

              <div class="game-card__footer">
                <div class="game-card__score">
                  <span>
                    {{ game.id === 'lucky' ? t('gameCenter.scores.credits') : t('gameCenter.labels.bestScore') }}
                  </span>
                  <strong>
                    {{ game.id === 'lucky'
                      ? formatCredits(wallet?.credits ?? '0')
                      : formatScore(bestScores[game.id] ?? 0) }}
                  </strong>
                </div>
                <span class="game-card__launch" aria-hidden="true">
                  <Icon name="arrowRight" size="sm" :stroke-width="2" />
                </span>
              </div>
            </div>
          </RouterLink>
        </div>
      </section>

      <nav class="quick-links" :aria-label="t('gameCenter.quickLinks.label')" data-testid="game-quick-links">
        <RouterLink to="/profile" class="quick-link">
          <Icon name="user" size="sm" />
          <span>{{ t('gameCenter.quickLinks.profile') }}</span>
        </RouterLink>
        <a href="#game-checkin" class="quick-link">
          <Icon name="calendar" size="sm" />
          <span>{{ t('gameCenter.quickLinks.checkin') }}</span>
        </a>
		<a href="#game-gift" class="quick-link">
		  <Icon name="mail" size="sm" />
		  <span>{{ t('gameCenter.quickLinks.gift') }}</span>
		</a>
        <a href="#game-ledger" class="quick-link">
          <Icon name="clipboard" size="sm" />
          <span>{{ t('gameCenter.quickLinks.ledger') }}</span>
        </a>
        <RouterLink to="/redeem" class="quick-link">
          <Icon name="gift" size="sm" />
          <span>{{ t('gameCenter.quickLinks.redeem') }}</span>
        </RouterLink>
      </nav>

      <section class="leaderboard" aria-labelledby="leaderboard-title" data-testid="game-leaderboard">
        <div class="leaderboard__heading">
          <div>
            <h2 id="leaderboard-title">{{ t('gameCenter.leaderboard.title') }}</h2>
          </div>
          <span>{{ t('gameCenter.leaderboard.topTen') }}</span>
        </div>

        <div class="leaderboard-tabs" role="tablist" :aria-label="t('gameCenter.leaderboard.gameTabs')">
          <button
            v-for="game in leaderboardGames"
            :id="`leaderboard-tab-${game.id}`"
            :key="game.id"
            type="button"
            role="tab"
            class="leaderboard-tab"
            :class="{ 'leaderboard-tab--active': activeGameId === game.id }"
            :aria-selected="activeGameId === game.id"
            :aria-controls="`leaderboard-panel-${game.id}`"
            :data-testid="`leaderboard-tab-${game.id}`"
            @click="selectLeaderboardGame(game.id)"
          >
            {{ t(game.titleKey) }}
          </button>
        </div>

        <div
          :id="`leaderboard-panel-${activeGameId}`"
          class="leaderboard__panel"
          role="tabpanel"
          :aria-labelledby="`leaderboard-tab-${activeGameId}`"
        >
          <div v-if="leaderboardLoading" class="leaderboard-state" data-testid="leaderboard-loading">
            {{ t('gameCenter.leaderboard.loading') }}
          </div>
          <div v-else-if="leaderboardError" class="leaderboard-state leaderboard-state--error" role="alert">
            <span>{{ leaderboardError }}</span>
            <button type="button" data-testid="leaderboard-retry" @click="loadLeaderboard(activeGameId)">
              {{ t('gameCenter.leaderboard.retry') }}
            </button>
          </div>
          <div
            v-else-if="!leaderboard || leaderboard.items.length === 0"
            class="leaderboard-state"
            data-testid="leaderboard-empty"
          >
            {{ t('gameCenter.leaderboard.empty') }}
          </div>
          <template v-else>
            <div class="leaderboard-table-wrap">
              <table class="leaderboard-table">
                <thead>
                  <tr>
                    <th scope="col">{{ t('gameCenter.leaderboard.rank') }}</th>
                    <th scope="col">{{ t('gameCenter.leaderboard.user') }}</th>
                    <th scope="col">{{ t('gameCenter.leaderboard.score') }}</th>
                    <th scope="col">{{ t('gameCenter.leaderboard.achievedAt') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="item in leaderboard.items"
                    :key="`${item.rank}-${item.user_display}`"
                    :class="{ 'leaderboard-row--self': item.is_current_user }"
                    :data-testid="`leaderboard-row-${item.rank}`"
                  >
                    <td data-label="#">{{ item.rank }}</td>
                    <td :data-label="t('gameCenter.leaderboard.user')">
                      {{ item.user_display }}
                      <span v-if="item.is_current_user" class="leaderboard-self-label">
                        {{ t('gameCenter.leaderboard.me') }}
                      </span>
                    </td>
                    <td :data-label="t('gameCenter.leaderboard.score')">{{ formatLeaderboardScore(item.score) }}</td>
                    <td :data-label="t('gameCenter.leaderboard.achievedAt')">{{ formatDateTime(item.achieved_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div
              v-if="myLeaderboardEntryOutsideTopTen"
              class="leaderboard-my-entry"
              data-testid="leaderboard-my-entry"
            >
              <span>{{ t('gameCenter.leaderboard.myRank') }}</span>
              <strong>#{{ myLeaderboardEntryOutsideTopTen.rank }}</strong>
              <span>{{ formatLeaderboardScore(myLeaderboardEntryOutsideTopTen.score) }}</span>
              <time :datetime="myLeaderboardEntryOutsideTopTen.achieved_at">
                {{ formatDateTime(myLeaderboardEntryOutsideTopTen.achieved_at) }}
              </time>
            </div>
          </template>
        </div>
      </section>

      <section class="wallet-panel" aria-labelledby="game-wallet-title">
        <div class="wallet-panel__header">
          <div>
            <span class="wallet-panel__eyebrow">{{ t('gameCenter.wallet.eyebrow') }}</span>
            <h2 id="game-wallet-title">{{ t('gameCenter.wallet.title') }}</h2>
          </div>
          <button
            type="button"
            class="wallet-panel__refresh"
            :disabled="walletLoading || submittingCheckin || claimingRewardId !== null"
            :title="t('gameCenter.wallet.refresh')"
            :aria-label="t('gameCenter.wallet.refresh')"
            data-testid="game-wallet-refresh"
            @click="refreshAll"
          >
            <Icon name="refresh" size="sm" :class="{ 'wallet-panel__refresh-icon--active': walletLoading }" />
          </button>
        </div>

        <div v-if="walletLoading && !wallet" class="wallet-panel__loading" data-testid="game-wallet-loading">
          <span v-for="index in 4" :key="index"></span>
        </div>

        <div v-else-if="walletError && !wallet" class="wallet-panel__error" role="alert">
          <div>
            <strong>{{ t('gameCenter.wallet.loadFailed') }}</strong>
            <p>{{ walletError }}</p>
          </div>
          <button type="button" class="wallet-panel__retry" @click="loadWallet">
            {{ t('gameCenter.wallet.retry') }}
          </button>
        </div>

        <template v-else-if="wallet">
          <div class="wallet-stats">
            <div class="wallet-stat wallet-stat--credits">
              <span>{{ t('gameCenter.wallet.credits') }}</span>
              <strong data-testid="game-wallet-credits">{{ formatCredits(wallet.credits) }}</strong>
            </div>
            <div class="wallet-stat">
              <span>{{ t('gameCenter.wallet.balance') }}</span>
              <strong data-testid="game-wallet-balance">{{ formatBalance(wallet.account_balance) }}</strong>
            </div>
            <div class="wallet-stat">
              <span>{{ t('gameCenter.wallet.freeSpins') }}</span>
              <strong data-testid="game-wallet-free-spins">{{ formatCredits(wallet.free_spins_remaining) }}</strong>
            </div>
            <div class="wallet-stat wallet-stat--remaining">
              <span>{{ t('gameCenter.wallet.checkinStatus') }}</span>
              <strong data-testid="game-wallet-checkin-status">
                {{ wallet.checked_in_today ? t('gameCenter.wallet.checkinDone') : t('gameCenter.wallet.checkinPending') }}
              </strong>
            </div>
          </div>

          <div
            v-if="!loyaltyOpen"
            class="loyalty-notice"
            role="status"
            data-testid="loyalty-unavailable"
          >
            <Icon name="infoCircle" size="sm" />
            <div>
              <strong>{{ t('gameCenter.wallet.maintenanceTitle') }}</strong>
              <p>{{ t('gameCenter.wallet.maintenance') }}</p>
            </div>
          </div>

          <div class="loyalty-grid" :class="{ 'loyalty-grid--disabled': !loyaltyOpen }">
            <section id="game-checkin" class="checkin-card anchor-target" aria-labelledby="checkin-title">
              <div class="section-heading">
                <h3 id="checkin-title">{{ t('gameCenter.checkin.title') }}</h3>
                <p>{{ t('gameCenter.checkin.description') }}</p>
              </div>
              <p class="checkin-card__meta" data-testid="checkin-credits-hint">
                {{ t('gameCenter.wallet.checkinCredits', { credits: formatCredits(wallet.checkin_credits) }) }}
              </p>
              <button
                type="button"
                class="primary-action"
                data-testid="game-checkin-submit"
                :disabled="!canCheckIn"
                @click="submitCheckIn"
              >
                <span v-if="submittingCheckin" class="action-spinner" aria-hidden="true"></span>
                {{
                  wallet.checked_in_today
                    ? t('gameCenter.checkin.done')
                    : submittingCheckin
                      ? t('gameCenter.checkin.submitting')
                      : t('gameCenter.checkin.button')
                }}
              </button>
              <p
                v-if="lastCheckinAward"
                class="checkin-card__success"
                role="status"
                data-testid="checkin-success"
              >
                {{ t('gameCenter.checkin.success', { credits: formatCredits(lastCheckinAward) }) }}
              </p>
            </section>

            <section class="rewards-card" aria-labelledby="rewards-title">
              <div class="section-heading">
                <div>
                  <h3 id="rewards-title">{{ t('gameCenter.rewards.title') }}</h3>
                  <p>{{ t('gameCenter.rewards.description') }}</p>
                </div>
                <span
                  v-if="rewards"
                  class="rewards-card__limit"
                  data-testid="rewards-daily-limit"
                >
                  {{
                    t('gameCenter.rewards.dailyLimit', {
                      used: String(rewards.claimed_today),
                      limit: rewards.daily_reward_limit === 0
                        ? t('gameCenter.wallet.unlimited')
                        : String(rewards.daily_reward_limit),
                    })
                  }}
                </span>
              </div>

              <div v-if="rewardsLoading && !rewards" class="inline-loading" data-testid="rewards-loading">
                {{ t('gameCenter.rewards.loadFailed') }}
              </div>
              <div v-else-if="rewardsError && !rewards" class="inline-error" role="alert">
                <span>{{ rewardsError }}</span>
                <button type="button" @click="loadRewards">{{ t('gameCenter.wallet.retry') }}</button>
              </div>
              <div
                v-else-if="rewards && rewards.items.length === 0"
                class="empty-state"
                data-testid="rewards-empty"
              >
                {{ t('gameCenter.rewards.empty') }}
              </div>
              <ul v-else-if="rewards" class="reward-list" data-testid="reward-list">
                <li
                  v-for="item in rewards.items"
                  :key="item.id"
                  class="reward-item"
                  :data-testid="`reward-item-${item.id}`"
                >
                  <div class="reward-item__body">
                    <h4>{{ item.title }}</h4>
                    <dl>
                      <div>
                        <dt>{{ t('gameCenter.rewards.creditCost') }}</dt>
                        <dd>{{ formatCredits(item.credit_cost) }}</dd>
                      </div>
                      <div>
                        <dt>{{ t('gameCenter.rewards.voucherValue') }}</dt>
                        <dd>{{ formatBalance(item.voucher_value) }}</dd>
                      </div>
                      <div>
                        <dt>{{ t('gameCenter.rewards.remainingToday') }}</dt>
                        <dd data-testid="reward-remaining">
                          {{ formatRemainingStock(item.remaining_stock) }}
                        </dd>
                      </div>
                      <div>
                        <dt>{{ t('gameCenter.rewards.expiryDays', { days: item.expiry_days }) }}</dt>
                        <dd>
                          {{
                            item.expiry_days > 0
                              ? t('gameCenter.rewards.expiryDays', { days: item.expiry_days })
                              : t('gameCenter.rewards.noExpiry')
                          }}
                        </dd>
                      </div>
                    </dl>
                  </div>
                  <button
                    type="button"
                    class="primary-action primary-action--compact"
                    :data-testid="`reward-claim-${item.id}`"
                    :disabled="!canClaimReward(item)"
                    @click="submitClaim(item)"
                  >
                    <span
                      v-if="claimingRewardId === item.id"
                      class="action-spinner"
                      aria-hidden="true"
                    ></span>
                    {{
                      isStockEmpty(item)
                        ? t('gameCenter.rewards.stockEmpty')
                        : claimingRewardId === item.id
                          ? t('gameCenter.rewards.claiming')
                          : t('gameCenter.rewards.claim')
                    }}
                  </button>
                </li>
              </ul>

              <div
                v-if="lastClaim"
                class="claim-result"
                role="status"
                data-testid="reward-claim-result"
              >
                <div>
                  <strong>{{ t('gameCenter.rewards.success') }}</strong>
                  <p>
                    {{ t('gameCenter.rewards.redeemCode') }}：
                    <code data-testid="reward-redeem-code">{{ lastClaim.redeem_code }}</code>
                  </p>
                </div>
                <button
                  type="button"
                  class="secondary-action"
                  data-testid="reward-copy-code"
                  @click="copyRedeemCode(lastClaim.redeem_code)"
                >
                  {{ t('gameCenter.rewards.copyCode') }}
                </button>
              </div>
            </section>
          </div>

		  <section id="game-gift" class="gift-card anchor-target" aria-labelledby="gift-title">
			<div class="section-heading">
			  <h3 id="gift-title">{{ t('gameCenter.gift.title') }}</h3>
			</div>
			<form class="gift-form" data-testid="game-credit-gift-form" @submit.prevent="prepareGift">
			  <label class="gift-field" for="game-gift-email">
				<span>{{ t('gameCenter.gift.recipientEmail') }}</span>
				<input
				  id="game-gift-email"
				  v-model.trim="giftEmail"
				  type="email"
				  maxlength="320"
				  autocomplete="off"
				  :placeholder="t('gameCenter.gift.emailPlaceholder')"
				  :disabled="submittingGift || !loyaltyOpen"
				  data-testid="game-credit-gift-email"
				  required
				  @input="clearGiftConfirmation"
				/>
			  </label>
			  <label class="gift-field" for="game-gift-credits">
				<span>{{ t('gameCenter.gift.credits') }}</span>
				<input
				  id="game-gift-credits"
				  v-model.trim="giftCredits"
				  type="text"
				  inputmode="numeric"
				  pattern="[0-9]+"
				  maxlength="19"
				  :placeholder="t('gameCenter.gift.creditsPlaceholder')"
				  :disabled="submittingGift || !loyaltyOpen"
				  data-testid="game-credit-gift-credits"
				  required
				  @input="clearGiftConfirmation"
				/>
			  </label>
			  <button
				type="submit"
				class="primary-action gift-form__submit"
				data-testid="game-credit-gift-prepare"
				:disabled="!canPrepareGift"
			  >
				{{ t('gameCenter.gift.prepare') }}
			  </button>
			</form>

			<div v-if="giftConfirmation" class="gift-confirmation" role="status">
			  <p>
				{{ t('gameCenter.gift.confirmation', {
				  credits: formatCredits(giftConfirmation.credits),
				  email: giftConfirmation.email,
				}) }}
			  </p>
			  <div class="gift-confirmation__actions">
				<button
				  type="button"
				  class="secondary-action"
				  :disabled="submittingGift"
				  @click="giftConfirmation = null"
				>
				  {{ t('common.cancel') }}
				</button>
				<button
				  type="button"
				  class="primary-action"
				  data-testid="game-credit-gift-submit"
				  :disabled="submittingGift"
				  @click="submitGift"
				>
				  <span v-if="submittingGift" class="action-spinner" aria-hidden="true"></span>
				  {{ submittingGift ? t('gameCenter.gift.submitting') : t('gameCenter.gift.confirm') }}
				</button>
			  </div>
			</div>
			<p v-if="lastGift" class="gift-success" role="status" data-testid="game-credit-gift-success">
			  {{ t('gameCenter.gift.success', {
				credits: formatCredits(lastGift.credits),
				email: lastGift.recipient_email,
			  }) }}
			</p>
		  </section>

          <section id="game-ledger" class="ledger-card anchor-target" aria-labelledby="ledger-title">
            <div class="section-heading">
              <h3 id="ledger-title">{{ t('gameCenter.ledger.title') }}</h3>
              <p>{{ t('gameCenter.ledger.description') }}</p>
            </div>

            <div v-if="ledgerLoading && ledgerItems.length === 0" class="inline-loading">
              {{ t('gameCenter.ledger.loading') }}
            </div>
            <div v-else-if="ledgerError && ledgerItems.length === 0" class="inline-error" role="alert">
              <span>{{ ledgerError }}</span>
              <button type="button" @click="loadLedger(true)">{{ t('gameCenter.wallet.retry') }}</button>
            </div>
            <div
              v-else-if="ledgerItems.length === 0"
              class="empty-state"
              data-testid="ledger-empty"
            >
              {{ t('gameCenter.ledger.empty') }}
            </div>
            <ul v-else class="ledger-list" data-testid="ledger-list">
              <li
                v-for="entry in ledgerItems"
                :key="entry.id"
                class="ledger-item"
                :data-testid="`ledger-item-${entry.id}`"
              >
                <div>
                  <strong>{{ entryTypeLabel(entry.entry_type) }}</strong>
                  <time :datetime="entry.created_at">{{ formatDateTime(entry.created_at) }}</time>
                </div>
                <span
                  class="ledger-item__amount"
                  :class="{ 'ledger-item__amount--positive': isPositiveAmount(entry.amount) }"
                >
                  {{ formatSignedCredits(entry.amount) }}
                </span>
                <span class="ledger-item__balance">
                  {{ formatCredits(entry.credits_after) }}
                </span>
              </li>
            </ul>
            <button
              v-if="ledgerNextBeforeId"
              type="button"
              class="secondary-action ledger-card__more"
              data-testid="ledger-load-more"
              :disabled="ledgerLoading"
              @click="loadLedger(false)"
            >
              {{ ledgerLoading ? t('gameCenter.ledger.loading') : t('gameCenter.ledger.loadMore') }}
            </button>
          </section>
        </template>
      </section>

    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onActivated, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import GameCardArt from '@/features/game-center/components/GameCardArt.vue'
import { GAME_CATALOG } from '@/features/game-center/catalog'
import { readBestScore } from '@/features/game-center/storage'
import type { GameId } from '@/features/game-center/types'
import {
  checkInGameWallet,
  claimGameWalletReward,
  createGameWalletIdempotencyKey,
  getGameLeaderboard,
  getGameWallet,
  getGameWalletRewards,
	  getGameWalletTransactions,
	  giftGameWalletCredits,
	} from '@/api/gameWallet'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorCode } from '@/utils/apiError'
import type {
	  GameWallet,
	  GameCreditGiftResult,
  GameLeaderboardEntry,
  GameLeaderboardResult,
  GameWalletRewardClaimResult,
  GameWalletRewardItem,
  GameWalletRewardList,
  GameWalletTransaction,
} from '@/types'
import arcadeHallUrl from '@/assets/game-center/arcade-hall.webp'

const { locale, t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const games = GAME_CATALOG
const leaderboardGames = [
  ...GAME_CATALOG.filter((game) => game.id === 'lucky'),
  ...GAME_CATALOG.filter((game) => game.id !== 'lucky'),
]
const bestScores = reactive<Partial<Record<GameId, number>>>({})

const wallet = ref<GameWallet | null>(null)
const walletLoading = ref(false)
const walletError = ref('')
const rewards = ref<GameWalletRewardList | null>(null)
const rewardsLoading = ref(false)
const rewardsError = ref('')
const ledgerItems = ref<GameWalletTransaction[]>([])
const ledgerNextBeforeId = ref<string | null>(null)
const ledgerLoading = ref(false)
const ledgerError = ref('')
const submittingCheckin = ref(false)
const claimingRewardId = ref<string | null>(null)
const pendingCheckinKey = ref<string | null>(null)
const pendingClaimKeys = ref<Record<string, string>>({})
const lastCheckinAward = ref<string | null>(null)
const lastClaim = ref<GameWalletRewardClaimResult | null>(null)
const giftEmail = ref('')
const giftCredits = ref('')
const giftConfirmation = ref<{ email: string; credits: string } | null>(null)
const submittingGift = ref(false)
const pendingGiftRequest = ref<{ fingerprint: string; key: string } | null>(null)
const lastGift = ref<GameCreditGiftResult | null>(null)
const activeGameId = ref<GameId>('lucky')
const leaderboard = ref<GameLeaderboardResult | null>(null)
const leaderboardLoading = ref(false)
const leaderboardError = ref('')
let leaderboardRequestId = 0

const myLeaderboardEntryOutsideTopTen = computed<GameLeaderboardEntry | null>(() => {
  const result = leaderboard.value
  if (!result?.my_entry || result.items.some((item) => item.is_current_user)) return null
  return result.my_entry
})

const gameLoyaltyErrorKeys: Record<string, string> = {
  GAME_LOYALTY_DISABLED: 'gameCenter.apiErrors.disabled',
  GAME_LOYALTY_CONFIG_INVALID: 'gameCenter.apiErrors.configInvalid',
  GAME_LOYALTY_NOT_CONFIGURED: 'gameCenter.apiErrors.notConfigured',
  GAME_LOYALTY_CHECKIN_ALREADY_CLAIMED: 'gameCenter.apiErrors.alreadyCheckedIn',
  GAME_LOYALTY_INSUFFICIENT_CREDITS: 'gameCenter.apiErrors.insufficientCredits',
  GAME_LOYALTY_CREDITS_LIMIT_EXCEEDED: 'gameCenter.apiErrors.creditsLimit',
  GAME_LOYALTY_REWARD_NOT_FOUND: 'gameCenter.apiErrors.rewardNotFound',
  GAME_LOYALTY_REWARD_CAP_EXCEEDED: 'gameCenter.apiErrors.rewardLimit',
  GAME_LOYALTY_REWARD_STOCK_EXCEEDED: 'gameCenter.apiErrors.rewardStock',
  GAME_LOYALTY_REWARD_ID_INVALID: 'gameCenter.apiErrors.invalidReward',
  GAME_LOYALTY_BET_INVALID: 'gameCenter.apiErrors.invalidBet',
  GAME_LOYALTY_RNG_FAILED: 'gameCenter.apiErrors.rngFailed',
  GAME_LOYALTY_SPIN_LIMIT_EXCEEDED: 'gameCenter.apiErrors.spinLimit',
	GAME_CREDIT_GIFT_RECIPIENT_INVALID: 'gameCenter.apiErrors.giftRecipientInvalid',
	GAME_CREDIT_GIFT_RECIPIENT_NOT_FOUND: 'gameCenter.apiErrors.giftRecipientNotFound',
	GAME_CREDIT_GIFT_SELF_FORBIDDEN: 'gameCenter.apiErrors.giftSelfForbidden',
	GAME_CREDIT_GIFT_AMOUNT_INVALID: 'gameCenter.apiErrors.giftAmountInvalid',
	GAME_CREDIT_GIFT_UNAVAILABLE: 'gameCenter.apiErrors.giftUnavailable',
  IDEMPOTENCY_KEY_REQUIRED: 'gameCenter.apiErrors.requestInvalid',
  IDEMPOTENCY_KEY_INVALID: 'gameCenter.apiErrors.requestInvalid',
  IDEMPOTENCY_KEY_CONFLICT: 'gameCenter.apiErrors.requestConflict',
}

function gameLoyaltyErrorMessage(error: unknown, fallbackKey: string): string {
  const code = extractApiErrorCode(error)
  const messageKey = code ? gameLoyaltyErrorKeys[code] : undefined
  return t(messageKey ?? fallbackKey)
}

const loyaltyOpen = computed(() => Boolean(
  wallet.value?.loyalty_enabled && wallet.value.loyalty_available,
))

const canCheckIn = computed(() =>
  loyaltyOpen.value
  && !submittingCheckin.value
  && !wallet.value?.checked_in_today,
)

const canPrepareGift = computed(() => {
	if (!loyaltyOpen.value || submittingGift.value) return false
	const email = giftEmail.value.trim()
	if (!/^[^\s@]+@[^\s@]+$/.test(email)) return false
	const amount = parseUnsignedInteger(giftCredits.value)
	const available = parseUnsignedInteger(wallet.value?.credits ?? '0')
	return amount !== null && amount > 0n && available !== null && amount <= available
})

function parseUnsignedInteger(raw: string | number | bigint): bigint | null {
  const value = String(raw).trim()
  if (!/^-?\d+$/.test(value)) return null
  try {
    return BigInt(value)
  } catch {
    return null
  }
}

function loadBestScores(): void {
  games.forEach((game) => {
    bestScores[game.id] = readBestScore(game.id)
  })
}

function formatScore(score: number): string {
  return new Intl.NumberFormat(locale.value).format(score)
}

function formatCredits(credits: bigint | string | number): string {
  const value = typeof credits === 'bigint' ? credits : parseUnsignedInteger(credits)
  return value === null ? '--' : new Intl.NumberFormat(locale.value).format(value)
}

function formatLeaderboardScore(score: string): string {
  const value = parseUnsignedInteger(score)
  return value === null ? score : new Intl.NumberFormat(locale.value).format(value)
}

function formatSignedCredits(amount: string): string {
  const value = parseUnsignedInteger(amount)
  if (value === null) return '--'
  const formatted = new Intl.NumberFormat(locale.value).format(value < 0n ? -value : value)
  if (value > 0n) return `+${formatted}`
  if (value < 0n) return `-${formatted}`
  return formatted
}

function isPositiveAmount(amount: string): boolean {
  const value = parseUnsignedInteger(amount)
  return value !== null && value > 0n
}

function formatBalance(balance: number | string): string {
  const numeric = Number(balance)
  if (!Number.isFinite(numeric)) return String(balance)
  return new Intl.NumberFormat(locale.value, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(numeric)
}

function formatRemainingStock(remaining: number | null): string {
  if (remaining === null) return t('gameCenter.wallet.unlimited')
  return new Intl.NumberFormat(locale.value).format(remaining)
}

function formatDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale.value, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function entryTypeLabel(entryType: string): string {
  const key = `gameCenter.ledger.types.${entryType}`
  const labeled = t(key)
  return labeled === key ? t('gameCenter.ledger.types.unknown') : labeled
}

function isStockEmpty(item: GameWalletRewardItem): boolean {
  return item.remaining_stock !== null && item.remaining_stock <= 0
}

function canClaimReward(item: GameWalletRewardItem): boolean {
  if (!loyaltyOpen.value || claimingRewardId.value !== null) return false
  if (isStockEmpty(item)) return false
  const cost = parseUnsignedInteger(item.credit_cost)
  const credits = parseUnsignedInteger(wallet.value?.credits ?? '0')
  if (cost === null || credits === null || credits < cost) return false
  return true
}

async function loadWallet(): Promise<void> {
  walletLoading.value = true
  walletError.value = ''
  try {
    wallet.value = await getGameWallet()
  } catch (error: unknown) {
    walletError.value = gameLoyaltyErrorMessage(error, 'gameCenter.wallet.loadFailed')
  } finally {
    walletLoading.value = false
  }
}

async function loadRewards(): Promise<void> {
  rewardsLoading.value = true
  rewardsError.value = ''
  try {
    rewards.value = await getGameWalletRewards()
  } catch (error: unknown) {
    rewardsError.value = gameLoyaltyErrorMessage(error, 'gameCenter.rewards.loadFailed')
  } finally {
    rewardsLoading.value = false
  }
}

async function loadLedger(reset: boolean): Promise<void> {
  if (ledgerLoading.value) return
  ledgerLoading.value = true
  ledgerError.value = ''
  try {
    const query = reset || !ledgerNextBeforeId.value
      ? { limit: 20 }
      : { limit: 20, before_id: ledgerNextBeforeId.value }
    const result = await getGameWalletTransactions(query)
    ledgerItems.value = reset ? result.items : [...ledgerItems.value, ...result.items]
    ledgerNextBeforeId.value = result.next_before_id
  } catch (error: unknown) {
    ledgerError.value = gameLoyaltyErrorMessage(error, 'gameCenter.ledger.loadFailed')
  } finally {
    ledgerLoading.value = false
  }
}

async function loadLeaderboard(gameId: GameId): Promise<void> {
  const requestId = ++leaderboardRequestId
  leaderboardLoading.value = true
  leaderboardError.value = ''
  leaderboard.value = null
  try {
    const result = await getGameLeaderboard({ game_id: gameId, limit: 10 })
    if (requestId === leaderboardRequestId && activeGameId.value === gameId) {
      leaderboard.value = result
    }
  } catch {
    if (requestId === leaderboardRequestId && activeGameId.value === gameId) {
      leaderboardError.value = t('gameCenter.leaderboard.loadFailed')
    }
  } finally {
    if (requestId === leaderboardRequestId) {
      leaderboardLoading.value = false
    }
  }
}

function selectLeaderboardGame(gameId: GameId): void {
  if (activeGameId.value === gameId) return
  activeGameId.value = gameId
  void loadLeaderboard(gameId)
}

async function refreshAll(): Promise<void> {
  await Promise.allSettled([loadWallet(), loadRewards(), loadLedger(true)])
}

async function submitCheckIn(): Promise<void> {
  if (!canCheckIn.value || submittingCheckin.value) return
  submittingCheckin.value = true
  try {
    if (!pendingCheckinKey.value) {
      pendingCheckinKey.value = createGameWalletIdempotencyKey()
    }
    const result = await checkInGameWallet(pendingCheckinKey.value)
    pendingCheckinKey.value = null
    lastCheckinAward.value = result.credits_awarded
    if (wallet.value) {
      wallet.value = {
        ...wallet.value,
        credits: result.credits_after,
        checked_in_today: true,
        today_checkin_at: result.created_at,
      }
    }
    appStore.showSuccess(t('gameCenter.checkin.success', {
      credits: formatCredits(result.credits_awarded),
    }))
    await Promise.allSettled([loadLedger(true), authStore.refreshUser()])
  } catch (error: unknown) {
    appStore.showError(gameLoyaltyErrorMessage(error, 'gameCenter.checkin.failed'))
  } finally {
    submittingCheckin.value = false
  }
}

async function submitClaim(item: GameWalletRewardItem): Promise<void> {
  if (!canClaimReward(item)) return
  claimingRewardId.value = item.id
  try {
    if (!pendingClaimKeys.value[item.id]) {
      pendingClaimKeys.value = {
        ...pendingClaimKeys.value,
        [item.id]: createGameWalletIdempotencyKey(),
      }
    }
    const key = pendingClaimKeys.value[item.id]
    const result = await claimGameWalletReward({ reward_id: item.id }, key)
    const nextKeys = { ...pendingClaimKeys.value }
    delete nextKeys[item.id]
    pendingClaimKeys.value = nextKeys
    lastClaim.value = result
    if (wallet.value) {
      wallet.value = {
        ...wallet.value,
        credits: result.credits_after,
      }
    }
    appStore.showSuccess(t('gameCenter.rewards.success'))
    await Promise.allSettled([loadRewards(), loadLedger(true), authStore.refreshUser()])
  } catch (error: unknown) {
    appStore.showError(gameLoyaltyErrorMessage(error, 'gameCenter.rewards.failed'))
  } finally {
    claimingRewardId.value = null
  }
}

function clearGiftConfirmation(): void {
	giftConfirmation.value = null
	lastGift.value = null
}

function prepareGift(): void {
	if (!canPrepareGift.value) return
	const amount = parseUnsignedInteger(giftCredits.value)
	if (amount === null || amount <= 0n) return
	giftConfirmation.value = {
		email: giftEmail.value.trim().toLowerCase(),
		credits: amount.toString(),
	}
}

async function submitGift(): Promise<void> {
	const request = giftConfirmation.value
	if (!request || submittingGift.value) return
	const fingerprint = `${request.email}\n${request.credits}`
	if (!pendingGiftRequest.value || pendingGiftRequest.value.fingerprint !== fingerprint) {
		pendingGiftRequest.value = { fingerprint, key: createGameWalletIdempotencyKey() }
	}
	submittingGift.value = true
	try {
		const result = await giftGameWalletCredits(
			{ recipient_email: request.email, credits: request.credits },
			pendingGiftRequest.value.key,
		)
		pendingGiftRequest.value = null
		lastGift.value = result
		giftConfirmation.value = null
		giftEmail.value = ''
		giftCredits.value = ''
		if (wallet.value) {
			wallet.value = { ...wallet.value, credits: result.sender_credits_after }
		}
		appStore.showSuccess(t('gameCenter.gift.success', {
			credits: formatCredits(result.credits),
			email: result.recipient_email,
		}))
		await loadLedger(true)
	} catch (error: unknown) {
		appStore.showError(gameLoyaltyErrorMessage(error, 'gameCenter.gift.failed'))
	} finally {
		submittingGift.value = false
	}
}

async function copyRedeemCode(code: string): Promise<void> {
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(code)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = code
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    appStore.showSuccess(t('gameCenter.rewards.copied'))
  } catch {
    appStore.showError(t('gameCenter.rewards.copyFailed'))
  }
}

onMounted(() => {
  loadBestScores()
  void refreshAll()
  void loadLeaderboard(activeGameId.value)
})

onActivated(() => {
  loadBestScores()
  if (wallet.value) {
    void refreshAll()
    void loadLeaderboard(activeGameId.value)
  }
})
</script>

<style scoped>
.game-center {
  --arcade-border: rgba(148, 163, 184, 0.2);
  width: 100%;
  max-width: 1480px;
  margin: 0 auto;
}

.arcade-hero {
  position: relative;
  display: flex;
  height: clamp(190px, 24vw, 280px);
  min-height: 190px;
  overflow: hidden;
  align-items: flex-end;
  justify-content: space-between;
  border: 1px solid rgba(34, 211, 238, 0.3);
  border-radius: 8px;
  background-color: #080b11;
  background-position: center 58%;
  background-size: cover;
  box-shadow: 0 16px 38px rgba(2, 6, 23, 0.24);
}

.arcade-hero__shade {
  position: absolute;
  inset: 0;
  background: rgba(2, 6, 14, 0.56);
}

.arcade-hero__content,
.arcade-hero__counter {
  position: relative;
  z-index: 1;
}

.arcade-hero__content {
  padding: 24px;
}

.arcade-hero__brand {
  display: block;
  margin-bottom: 6px;
  color: #67e8f9;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0;
}

.arcade-hero h1 {
  margin: 0;
  color: #f8fafc;
  font-size: 48px;
  font-weight: 800;
  line-height: 1.02;
  letter-spacing: 0;
  text-shadow: 0 0 26px rgba(34, 211, 238, 0.38);
}

.arcade-hero__counter {
  display: grid;
  width: 70px;
  height: 52px;
  margin: 0 24px 24px 0;
  place-items: center;
  border: 1px solid rgba(244, 114, 182, 0.65);
  border-radius: 6px;
  background: rgba(3, 7, 18, 0.84);
  color: #f9a8d4;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 22px;
  font-weight: 900;
  box-shadow: 0 0 22px rgba(244, 114, 182, 0.18);
}

.quick-links {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 1px;
  overflow: hidden;
  margin-top: 18px;
  border: 1px solid #dbe4ee;
  border-radius: 8px;
  background: #dbe4ee;
}

:global(.dark) .quick-links {
  border-color: rgba(148, 163, 184, 0.2);
  background: rgba(148, 163, 184, 0.2);
}

.quick-link {
  display: flex;
  min-width: 0;
  min-height: 48px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 12px;
  background: #ffffff;
  color: #334155;
  font-size: 13px;
  font-weight: 750;
  text-decoration: none;
}

.quick-link:hover {
  background: #f8fafc;
  color: #0f766e;
}

.quick-link span {
  min-width: 0;
  overflow-wrap: anywhere;
}

:global(.dark) .quick-link {
  background: #101620;
  color: #e2e8f0;
}

:global(.dark) .quick-link:hover {
  background: #161e2b;
  color: #5eead4;
}

.leaderboard {
  overflow: hidden;
  margin-top: 18px;
  border: 1px solid #dbe4ee;
  border-radius: 8px;
  background: #ffffff;
}

:global(.dark) .leaderboard {
  border-color: rgba(148, 163, 184, 0.2);
  background: #101620;
}

.leaderboard__heading {
  display: flex;
  min-height: 64px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 18px;
  border-bottom: 1px solid #e5e7eb;
}

:global(.dark) .leaderboard__heading {
  border-bottom-color: rgba(148, 163, 184, 0.16);
}

.leaderboard__heading h2 {
  margin: 0;
  color: #111827;
  font-size: 18px;
  font-weight: 800;
  letter-spacing: 0;
}

:global(.dark) .leaderboard__heading h2 {
  color: #f8fafc;
}

.leaderboard__heading p,
.leaderboard__heading > span {
  margin: 3px 0 0;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
}

.leaderboard-tabs {
  display: flex;
  gap: 4px;
  overflow-x: auto;
  padding: 8px 10px;
  border-bottom: 1px solid #e5e7eb;
  scrollbar-width: thin;
}

:global(.dark) .leaderboard-tabs {
  border-bottom-color: rgba(148, 163, 184, 0.16);
}

.leaderboard-tab {
  min-height: 34px;
  flex: none;
  border: 1px solid transparent;
  border-radius: 6px;
  padding: 0 12px;
  background: transparent;
  color: #64748b;
  font-size: 12px;
  font-weight: 750;
  cursor: pointer;
  white-space: nowrap;
}

.leaderboard-tab:hover {
  color: #0f766e;
}

.leaderboard-tab--active {
  border-color: rgba(15, 118, 110, 0.25);
  background: rgba(15, 118, 110, 0.09);
  color: #0f766e;
}

:global(.dark) .leaderboard-tab--active {
  border-color: rgba(94, 234, 212, 0.25);
  background: rgba(94, 234, 212, 0.09);
  color: #5eead4;
}

.leaderboard__panel {
  min-height: 152px;
}

.leaderboard-state {
  display: flex;
  min-height: 152px;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 24px;
  color: #64748b;
  font-size: 13px;
}

.leaderboard-state--error {
  color: #b91c1c;
}

.leaderboard-state button {
  border: 0;
  background: transparent;
  color: inherit;
  font-weight: 800;
  cursor: pointer;
}

.leaderboard-table-wrap {
  overflow-x: auto;
}

.leaderboard-table {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  color: #334155;
  font-size: 12px;
  table-layout: fixed;
}

:global(.dark) .leaderboard-table {
  color: #e2e8f0;
}

.leaderboard-table th,
.leaderboard-table td {
  padding: 10px 16px;
  border-bottom: 1px solid #eef2f7;
  overflow-wrap: anywhere;
  text-align: left;
}

:global(.dark) .leaderboard-table th,
:global(.dark) .leaderboard-table td {
  border-bottom-color: rgba(148, 163, 184, 0.12);
}

.leaderboard-table th:first-child,
.leaderboard-table td:first-child {
  width: 72px;
  text-align: center;
}

.leaderboard-table th:nth-child(3),
.leaderboard-table td:nth-child(3) {
  width: 160px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-variant-numeric: tabular-nums;
}

.leaderboard-table th:last-child,
.leaderboard-table td:last-child {
  width: 170px;
}

.leaderboard-table th {
  color: #64748b;
  font-size: 10px;
  font-weight: 800;
}

.leaderboard-row--self {
  background: rgba(15, 118, 110, 0.08);
}

.leaderboard-self-label {
  margin-left: 6px;
  color: #0f766e;
  font-size: 10px;
  font-weight: 800;
}

:global(.dark) .leaderboard-self-label {
  color: #5eead4;
}

.leaderboard-my-entry {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto auto;
  align-items: center;
  gap: 16px;
  padding: 11px 16px;
  border-top: 1px solid rgba(15, 118, 110, 0.2);
  background: rgba(15, 118, 110, 0.08);
  color: #334155;
  font-size: 12px;
}

:global(.dark) .leaderboard-my-entry {
  color: #e2e8f0;
}

.leaderboard-my-entry strong {
  color: #0f766e;
  font-size: 14px;
}

:global(.dark) .leaderboard-my-entry strong {
  color: #5eead4;
}

.anchor-target {
  scroll-margin-top: 24px;
}

.wallet-panel {
  overflow: hidden;
  margin-top: 18px;
  border: 1px solid #dbe4ee;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
}

:global(.dark) .wallet-panel {
  border-color: rgba(148, 163, 184, 0.2);
  background: #101620;
}

.wallet-panel__header {
  display: flex;
  min-height: 76px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid #e5e7eb;
}

:global(.dark) .wallet-panel__header {
  border-bottom-color: rgba(148, 163, 184, 0.16);
}

.wallet-panel__eyebrow {
  display: block;
  margin-bottom: 3px;
  color: #0f766e;
  font-size: 10px;
  font-weight: 800;
  text-transform: uppercase;
}

:global(.dark) .wallet-panel__eyebrow {
  color: #5eead4;
}

.wallet-panel h2,
.section-heading h3,
.reward-item h4 {
  margin: 0;
  color: #111827;
  letter-spacing: 0;
}

.wallet-panel h2 {
  font-size: 18px;
  font-weight: 800;
}

:global(.dark) .wallet-panel h2,
:global(.dark) .section-heading h3,
:global(.dark) .reward-item h4 {
  color: #f8fafc;
}

.wallet-panel__refresh {
  display: grid;
  width: 36px;
  height: 36px;
  flex: none;
  place-items: center;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  background: transparent;
  color: #475569;
  cursor: pointer;
}

.wallet-panel__refresh:hover:not(:disabled) {
  border-color: #0f766e;
  color: #0f766e;
}

.wallet-panel__refresh:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.wallet-panel__refresh-icon--active {
  animation: wallet-spin 800ms linear infinite;
}

.wallet-panel__loading,
.wallet-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.wallet-panel__loading {
  gap: 1px;
  background: #e5e7eb;
}

.wallet-panel__loading span {
  height: 84px;
  background: #f1f5f9;
  animation: wallet-pulse 1.3s ease-in-out infinite;
}

:global(.dark) .wallet-panel__loading {
  background: rgba(148, 163, 184, 0.14);
}

:global(.dark) .wallet-panel__loading span {
  background: #161e2b;
}

.wallet-panel__error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 18px 20px;
  color: #991b1b;
}

.wallet-panel__error strong {
  font-size: 14px;
}

.wallet-panel__error p {
  margin: 3px 0 0;
  font-size: 12px;
}

.wallet-panel__retry {
  flex: none;
  border: 0;
  background: transparent;
  color: #b91c1c;
  font-size: 13px;
  font-weight: 750;
  cursor: pointer;
}

.wallet-stats {
  border-bottom: 1px solid #e5e7eb;
}

:global(.dark) .wallet-stats {
  border-bottom-color: rgba(148, 163, 184, 0.16);
}

.wallet-stat {
  display: grid;
  min-width: 0;
  min-height: 88px;
  align-content: center;
  gap: 5px;
  padding: 14px 20px;
  border-right: 1px solid #e5e7eb;
}

.wallet-stat:last-child {
  border-right: 0;
}

:global(.dark) .wallet-stat {
  border-right-color: rgba(148, 163, 184, 0.16);
}

.wallet-stat span {
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
}

.wallet-stat strong {
  overflow-wrap: anywhere;
  color: #111827;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 17px;
  font-weight: 850;
  font-variant-numeric: tabular-nums;
  min-height: 1.2em;
}

.wallet-stat--credits strong {
  color: #0f766e;
}

.wallet-stat--remaining strong {
  color: #b45309;
}

:global(.dark) .wallet-stat strong {
  color: #f8fafc;
}

:global(.dark) .wallet-stat--credits strong {
  color: #5eead4;
}

:global(.dark) .wallet-stat--remaining strong {
  color: #fbbf24;
}

.loyalty-notice {
  display: flex;
  gap: 10px;
  margin: 16px 20px 0;
  padding: 12px 14px;
  border-left: 3px solid #d97706;
  background: #fffbeb;
  color: #92400e;
  font-size: 12px;
  line-height: 1.5;
}

.loyalty-notice p {
  margin: 4px 0 0;
}

:global(.dark) .loyalty-notice {
  background: rgba(146, 64, 14, 0.16);
  color: #fcd34d;
}

.loyalty-grid {
  display: grid;
  grid-template-columns: minmax(240px, 0.9fr) minmax(0, 1.4fr);
  gap: 1px;
  margin-top: 0;
  background: #e5e7eb;
}

:global(.dark) .loyalty-grid {
  background: rgba(148, 163, 184, 0.14);
}

.loyalty-grid--disabled {
  opacity: 0.72;
}

.checkin-card,
.rewards-card,
.gift-card,
.ledger-card {
  background: #ffffff;
  padding: 18px 20px 20px;
}

:global(.dark) .checkin-card,
:global(.dark) .rewards-card,
:global(.dark) .gift-card,
:global(.dark) .ledger-card {
  background: #101620;
}

.gift-card {
  border-top: 1px solid #e5e7eb;
}

:global(.dark) .gift-card {
  border-top-color: rgba(148, 163, 184, 0.16);
}

.gift-form {
  display: grid;
  grid-template-columns: minmax(220px, 1.5fr) minmax(150px, 0.7fr) auto;
  align-items: end;
  gap: 12px;
  margin-top: 14px;
}

.gift-field {
  display: grid;
  min-width: 0;
  gap: 6px;
  color: #475569;
  font-size: 11px;
  font-weight: 750;
}

:global(.dark) .gift-field {
  color: #cbd5e1;
}

.gift-field input {
  width: 100%;
  min-height: 42px;
  box-sizing: border-box;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  padding: 0 12px;
  background: #ffffff;
  color: #111827;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
}

.gift-field input:focus {
  border-color: #0f766e;
  outline: 2px solid rgba(15, 118, 110, 0.16);
  outline-offset: 1px;
}

.gift-field input:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

:global(.dark) .gift-field input {
  border-color: #334155;
  background: #0b1019;
  color: #f8fafc;
}

.gift-form__submit {
  margin-top: 0;
  min-width: 112px;
}

.gift-confirmation {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid rgba(217, 119, 6, 0.3);
  border-radius: 6px;
  background: #fffbeb;
  color: #92400e;
}

:global(.dark) .gift-confirmation {
  background: rgba(146, 64, 14, 0.16);
  color: #fcd34d;
}

.gift-confirmation p {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
}

.gift-confirmation__actions {
  display: flex;
  flex: none;
  gap: 8px;
}

.gift-confirmation__actions .primary-action {
  margin-top: 0;
}

.gift-success {
  margin: 12px 0 0;
  color: #0f766e;
  font-size: 13px;
  font-weight: 750;
}

:global(.dark) .gift-success {
  color: #5eead4;
}

.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.section-heading h3 {
  font-size: 15px;
  font-weight: 800;
}

.section-heading p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.checkin-card__meta,
.rewards-card__limit {
  margin: 12px 0 0;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.checkin-card__success {
  margin: 12px 0 0;
  color: #0f766e;
  font-size: 13px;
  font-weight: 700;
}

:global(.dark) .checkin-card__success {
  color: #5eead4;
}

.primary-action,
.secondary-action {
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 6px;
  padding: 0 18px;
  font-size: 13px;
  font-weight: 800;
  cursor: pointer;
}

.primary-action {
  margin-top: 14px;
  min-width: 132px;
  border: 1px solid #0f766e;
  background: #0f766e;
  color: #ffffff;
}

.primary-action--compact {
  margin-top: 0;
  min-height: 40px;
  min-width: 108px;
  flex: none;
}

.primary-action:hover:not(:disabled) {
  background: #115e59;
}

.primary-action:disabled,
.secondary-action:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.secondary-action {
  border: 1px solid #cbd5e1;
  background: transparent;
  color: #334155;
}

:global(.dark) .secondary-action {
  border-color: #334155;
  color: #e2e8f0;
}

.action-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: #ffffff;
  border-radius: 50%;
  animation: wallet-spin 700ms linear infinite;
}

.reward-list,
.ledger-list {
  list-style: none;
  margin: 14px 0 0;
  padding: 0;
  display: grid;
  gap: 10px;
}

.reward-item {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 14px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f8fafc;
}

:global(.dark) .reward-item {
  border-color: rgba(148, 163, 184, 0.16);
  background: #0b1019;
}

.reward-item h4 {
  font-size: 14px;
  font-weight: 800;
}

.reward-item dl {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 16px;
  margin: 10px 0 0;
}

.reward-item dt {
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
}

.reward-item dd {
  margin: 2px 0 0;
  color: #111827;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  font-weight: 750;
  font-variant-numeric: tabular-nums;
}

:global(.dark) .reward-item dd {
  color: #f8fafc;
}

.claim-result {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid rgba(15, 118, 110, 0.28);
  border-radius: 8px;
  background: rgba(15, 118, 110, 0.08);
}

.claim-result p {
  margin: 6px 0 0;
  font-size: 13px;
}

.claim-result code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-weight: 800;
  color: #0f766e;
  word-break: break-all;
}

:global(.dark) .claim-result code {
  color: #5eead4;
}

.ledger-card {
  border-top: 1px solid #e5e7eb;
}

:global(.dark) .ledger-card {
  border-top-color: rgba(148, 163, 184, 0.16);
}

.ledger-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  gap: 12px;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #eef2f7;
}

:global(.dark) .ledger-item {
  border-bottom-color: rgba(148, 163, 184, 0.12);
}

.ledger-item strong {
  display: block;
  color: #111827;
  font-size: 13px;
}

:global(.dark) .ledger-item strong {
  color: #f8fafc;
}

.ledger-item time {
  color: #94a3b8;
  font-size: 11px;
}

.ledger-item__amount,
.ledger-item__balance {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  font-weight: 750;
  font-variant-numeric: tabular-nums;
}

.ledger-item__amount {
  color: #b91c1c;
}

.ledger-item__amount--positive {
  color: #0f766e;
}

.ledger-item__balance {
  color: #64748b;
  min-width: 4.5ch;
  text-align: right;
}

.ledger-card__more {
  margin-top: 14px;
}

.inline-loading,
.inline-error,
.empty-state {
  margin-top: 14px;
  color: #64748b;
  font-size: 13px;
}

.inline-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: #b91c1c;
}

.inline-error button {
  border: 0;
  background: transparent;
  color: #b91c1c;
  font-weight: 750;
  cursor: pointer;
}

.arcade-catalog {
  margin-top: 22px;
}

.arcade-catalog__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.arcade-catalog__heading h2 {
  margin: 0;
  color: #111827;
  font-size: 18px;
  font-weight: 800;
}

:global(.dark) .arcade-catalog__heading h2 {
  color: #f8fafc;
}

.arcade-catalog__heading span {
  color: #94a3b8;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 14px;
  font-weight: 800;
}

.arcade-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 14px;
}

.game-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--arcade-border);
  border-radius: 8px;
  background: #0b1019;
  text-decoration: none;
  transition: transform 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.game-card:hover {
  transform: translateY(-2px);
  border-color: color-mix(in srgb, var(--game-accent) 55%, transparent);
  box-shadow: 0 14px 28px rgba(2, 6, 23, 0.28);
}

.game-card--featured {
  border-color: rgba(245, 158, 11, 0.45);
}

.game-card__art {
  height: 148px;
}

.game-card__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 14px 16px;
}

.game-card__copy h3 {
  margin: 0;
  color: #f8fafc;
  font-size: 15px;
  font-weight: 800;
}

.game-card__copy p {
  margin: 6px 0 0;
  color: #94a3b8;
  font-size: 12px;
  line-height: 1.45;
}

.game-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.game-card__score span {
  display: block;
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
}

.game-card__score strong {
  color: var(--game-accent);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 15px;
}

.game-card__launch {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 999px;
  background: var(--game-accent-soft);
  color: var(--game-accent);
}

@keyframes wallet-spin {
  to { transform: rotate(360deg); }
}

@keyframes wallet-pulse {
  0%, 100% { opacity: 0.55; }
  50% { opacity: 1; }
}

@media (max-width: 960px) {
  .wallet-panel__loading,
  .wallet-stats,
  .loyalty-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .arcade-hero h1 {
    font-size: 34px;
  }

  .gift-form {
    grid-template-columns: minmax(0, 1fr) minmax(150px, 0.55fr);
  }

  .gift-form__submit {
    grid-column: 1 / -1;
    justify-self: start;
  }
}

@media (max-width: 640px) {
  .quick-links {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .quick-link {
    justify-content: flex-start;
    padding-inline: 14px;
  }

  .leaderboard__heading {
    min-height: 58px;
    padding-inline: 14px;
  }

  .leaderboard-my-entry {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 6px 12px;
  }

  .leaderboard-my-entry time {
    grid-column: 1 / -1;
  }

  .wallet-panel__loading,
  .wallet-stats,
  .loyalty-grid,
  .reward-item dl {
    grid-template-columns: 1fr;
  }

  .wallet-stat,
  .reward-item {
    border-right: 0;
    border-bottom: 1px solid #e5e7eb;
  }

  :global(.dark) .wallet-stat,
  :global(.dark) .reward-item {
    border-bottom-color: rgba(148, 163, 184, 0.16);
  }

  .reward-item {
    flex-direction: column;
    align-items: stretch;
  }

  .primary-action,
  .secondary-action,
  .primary-action--compact {
    min-height: 52px;
    width: 100%;
  }

  .claim-result {
    flex-direction: column;
    align-items: stretch;
  }

  .gift-form {
    grid-template-columns: 1fr;
  }

  .gift-form__submit {
    grid-column: auto;
  }

  .gift-confirmation {
    flex-direction: column;
    align-items: stretch;
  }

  .gift-confirmation__actions {
    width: 100%;
  }

  .arcade-hero {
    height: 180px;
  }

  .arcade-hero h1 {
    font-size: 28px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .game-card,
  .wallet-panel__refresh-icon--active,
  .action-spinner,
  .wallet-panel__loading span {
    animation: none;
    transition: none;
  }
}
</style>
