<template>
  <svg class="os-mark" viewBox="0 0 24 24" aria-hidden="true">
    <!-- macOS: display with stand -->
    <g v-if="kind === 'mac'">
      <rect x="3" y="3" width="18" height="13" rx="2" fill="currentColor" />
      <rect x="5" y="5" width="14" height="9" rx="0.6" fill="none" stroke="#000" stroke-opacity="0.25" stroke-width="0.8" />
      <path d="M10 16h4l1.5 3.5h-7z" fill="currentColor" />
    </g>
    <!-- Windows: title-bar window -->
    <g v-else-if="kind === 'win'">
      <rect x="3" y="4" width="18" height="16" rx="1.5" fill="currentColor" />
      <rect x="3" y="4" width="18" height="4" rx="1.5" fill="#000" fill-opacity="0.28" />
      <rect x="5.5" y="10" width="5.5" height="4" rx="0.6" fill="#000" fill-opacity="0.22" />
      <rect x="13" y="10" width="5.5" height="4" rx="0.6" fill="#000" fill-opacity="0.22" />
    </g>
    <!-- Linux: terminal / Tux-free hex -->
    <g v-else-if="kind === 'linux'">
      <path d="M12 2.5 20 7v10l-8 4.5L4 17V7z" fill="currentColor" />
      <path d="M8 14.5c.8-2 2-3 4-3s3.2 1 4 3" fill="none" stroke="#000" stroke-opacity="0.35" stroke-width="1.4" stroke-linecap="round" />
      <circle cx="9.5" cy="10" r="1" fill="#000" fill-opacity="0.4" />
      <circle cx="14.5" cy="10" r="1" fill="#000" fill-opacity="0.4" />
    </g>
    <!-- Android: phone with chin bar -->
    <g v-else-if="kind === 'android'">
      <rect x="7" y="2" width="10" height="20" rx="2.2" fill="currentColor" />
      <rect x="9" y="4.5" width="6" height="12" rx="0.6" fill="#000" fill-opacity="0.22" />
      <rect x="10" y="18.2" width="4" height="1.4" rx="0.7" fill="#000" fill-opacity="0.35" />
    </g>
    <!-- iOS: squircle phone, punch-hole -->
    <g v-else-if="kind === 'ios'">
      <rect x="7" y="2" width="10" height="20" rx="3" fill="currentColor" />
      <rect x="9" y="5.2" width="6" height="11.5" rx="0.8" fill="#000" fill-opacity="0.22" />
      <rect x="10.2" y="3.3" width="3.6" height="1.1" rx="0.55" fill="#000" fill-opacity="0.35" />
    </g>
    <g v-else>
      <circle cx="12" cy="12" r="9" fill="currentColor" />
      <circle cx="12" cy="12" r="3" fill="#000" fill-opacity="0.3" />
    </g>
  </svg>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({ os: { type: String, default: "" } });

function osKind(os) {
  const s = (os || "").toLowerCase();
  if (s.includes("mac") || s === "darwin") return "mac";
  if (s.includes("win")) return "win";
  if (s.includes("android")) return "android";
  if (s.includes("ios") || s.includes("iphone") || s.includes("ipad")) return "ios";
  if (s.includes("linux")) return "linux";
  return "other";
}

const kind = computed(() => osKind(props.os));
</script>
