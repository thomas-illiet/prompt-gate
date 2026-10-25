<script setup lang="ts">
import loginGatewayImage from '~/assets/images/login-gateway.jpg'

const props = defineProps<{
  statusLabel: string
  statusTone: 'checking' | 'online' | 'unavailable'
}>()
</script>

<template>
  <section class="login-visual" aria-hidden="true">
    <img
      :src="loginGatewayImage"
      alt=""
      class="login-visual__image"
      data-test="login-visual-image"
      decoding="async"
      fetchpriority="high"
      height="1200"
      width="1200"
    />
    <div class="login-visual__wash" />
    <div
      class="login-visual__status"
      :class="`login-visual__status--${props.statusTone}`"
      data-test="login-visual-status"
      :data-tone="props.statusTone"
    >
      <span class="login-visual__status-ping" />
      <span>{{ props.statusLabel }}</span>
    </div>
    <span class="login-visual__sticker">Good routes only</span>
    <div class="login-visual__caption">
      <span class="login-visual__eyebrow">PromptGate, but friendlier</span>
      <strong>Serious guardrails.<br />Soft landing.</strong>
    </div>
  </section>
</template>

<style scoped>
.login-visual {
  position: relative;
  display: grid;
  min-height: 520px;
  place-items: center;
  overflow: hidden;
  background:
    radial-gradient(
      circle at 24% 18%,
      rgba(118, 236, 174, 0.22),
      transparent 25%
    ),
    radial-gradient(
      circle at 82% 76%,
      rgba(var(--app-accent-coral), 0.18),
      transparent 24%
    ),
    linear-gradient(145deg, rgb(16, 65, 47), rgb(7, 31, 24));
}

.login-visual::before {
  position: absolute;
  z-index: 1;
  inset: 0;
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px);
  background-size: 36px 36px;
  content: '';
  mask-image: radial-gradient(circle at center, #000, transparent 78%);
}

.login-visual__image {
  position: absolute;
  inset: 0;
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center 42%;
  animation: login-visual-arrive 720ms var(--app-ease-out) both;
}

.login-visual__wash {
  position: absolute;
  z-index: 1;
  inset: 0;
  background:
    linear-gradient(
      180deg,
      rgba(7, 31, 24, 0.03) 24%,
      rgba(7, 31, 24, 0.8) 100%
    ),
    linear-gradient(90deg, rgba(7, 31, 24, 0.32), transparent 62%);
}

.login-visual__status,
.login-visual__caption,
.login-visual__sticker {
  position: absolute;
  z-index: 2;
}

.login-visual__status {
  top: 22px;
  left: 22px;
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 13px;
  border: 1px solid rgba(255, 255, 255, 0.38);
  border-radius: 999px;
  background: rgba(255, 253, 246, 0.9);
  box-shadow:
    0 4px 0 rgba(7, 31, 24, 0.18),
    0 16px 32px -22px rgba(0, 0, 0, 0.7);
  color: rgb(13, 62, 45);
  font-size: 0.73rem;
  font-weight: 850;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  backdrop-filter: blur(12px);
}

.login-visual__status-ping {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgb(30, 174, 112);
  box-shadow: 0 0 0 0 rgba(30, 174, 112, 0.55);
  animation: login-status-ping 2.4s ease-out infinite;
}

.login-visual__status--checking .login-visual__status-ping {
  background: rgb(var(--v-theme-warning));
  box-shadow: 0 0 0 0 rgba(var(--v-theme-warning), 0.48);
}

.login-visual__status--unavailable .login-visual__status-ping {
  background: rgb(var(--v-theme-error));
  box-shadow: none;
  animation: none;
}

.login-visual__sticker {
  top: 26%;
  right: -24px;
  padding: 10px 30px 10px 16px;
  transform: rotate(5deg);
  border: 2px solid rgba(68, 45, 16, 0.24);
  border-radius: 11px 0 0 11px;
  background: rgb(var(--app-accent-butter));
  box-shadow: 0 7px 0 rgba(7, 31, 24, 0.2);
  color: rgb(67, 47, 20);
  font-size: 0.69rem;
  font-weight: 900;
  letter-spacing: 0.11em;
  text-transform: uppercase;
}

.login-visual__caption {
  right: clamp(22px, 4vw, 44px);
  bottom: clamp(24px, 4vw, 42px);
  left: clamp(22px, 4vw, 44px);
  display: grid;
  max-width: 31rem;
  gap: 7px;
  color: white;
  text-shadow: 0 2px 20px rgba(0, 0, 0, 0.32);
}

.login-visual__caption strong {
  font-size: clamp(1.8rem, 3.4vw, 3rem);
  line-height: 0.98;
  letter-spacing: -0.045em;
}

.login-visual__eyebrow {
  font-size: 0.72rem;
  font-weight: 850;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

@keyframes login-visual-arrive {
  from {
    opacity: 0;
    transform: scale(1.035);
  }
}

@keyframes login-status-ping {
  55%,
  100% {
    box-shadow: 0 0 0 9px rgba(30, 174, 112, 0);
  }
}

@media (max-width: 960px) {
  .login-visual {
    min-height: clamp(250px, 40vw, 360px);
  }

  .login-visual__sticker {
    top: 28%;
  }
}

@media (max-width: 600px) {
  .login-visual {
    min-height: 218px;
  }

  .login-visual__status {
    top: 14px;
    left: 14px;
  }

  .login-visual__sticker {
    display: none;
  }

  .login-visual__caption {
    right: 20px;
    bottom: 20px;
    left: 20px;
  }

  .login-visual__caption strong {
    font-size: 1.65rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .login-visual__image,
  .login-visual__status-ping {
    animation: none;
  }
}
</style>
