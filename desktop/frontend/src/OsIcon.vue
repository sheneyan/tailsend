<template>
  <img class="os-mark" :src="src" alt="" />
</template>

<script setup>
import { computed } from "vue";
import mac from "./assets/os/mac.png";
import win from "./assets/os/win.png";
import linux from "./assets/os/linux.png";
import android from "./assets/os/android.png";
import ios from "./assets/os/ios.png";
import other from "./assets/os/other.png";

const props = defineProps({ os: { type: String, default: "" } });

const files = { mac, win, linux, android, ios, other };

function osKind(os) {
  const s = (os || "").toLowerCase();
  if (s.includes("mac") || s === "darwin") return "mac";
  if (s.includes("win")) return "win";
  if (s.includes("android")) return "android";
  if (s.includes("ios") || s.includes("iphone") || s.includes("ipad")) return "ios";
  if (s.includes("linux")) return "linux";
  return "other";
}

const src = computed(() => files[osKind(props.os)]);
</script>
