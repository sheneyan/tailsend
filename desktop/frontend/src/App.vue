<template>
  <div class="shell" :class="{ darwin: platform === 'darwin' }" @dragover.prevent>
    <header class="top" style="--wails-draggable: drag">
      <div class="brand">
        <span class="logo">↑</span>
        <div>
          <div class="title">Tailsend</div>
          <div class="sub" v-if="snap.self?.hostname">{{ snap.self.hostname }}</div>
        </div>
      </div>
      <div class="pill" :class="pillClass">{{ pillLabel }}</div>
      <div class="actions" style="--wails-draggable: no-drag">
        <button class="ghost" @click="openPair" :disabled="snap.state !== 'Running'">Pair phone</button>
        <button class="ghost" @click="inboxOpen = !inboxOpen">
          Inbox
          <span class="badge" v-if="snap.inbox?.length">{{ snap.inbox.length }}</span>
        </button>
      </div>
    </header>

    <div class="banner" v-if="setupMessage">
      <strong>{{ setupTitle }}</strong>
      <p>{{ setupMessage }}</p>
      <a href="https://tailscale.com/download" target="_blank" rel="noreferrer">Install Tailscale</a>
      ·
      <a href="https://login.tailscale.com/admin/settings/general" target="_blank" rel="noreferrer">Enable Send Files</a>
    </div>
    <div class="banner warn" v-else-if="snap.error">{{ snap.error }}</div>

    <main class="main">
      <aside class="side">
        <div class="section-label">1. Files to send</div>
        <div class="drop" @click="pickFiles">
          <p v-if="!files.length">Drop files here, or click to choose.</p>
          <ul v-else>
            <li v-for="(f, i) in files" :key="f.path">
              <span class="fname">{{ f.name }}</span>
              <button class="x" @click.stop="files.splice(i, 1)">×</button>
            </li>
          </ul>
        </div>
        <div class="progress" v-if="progress">
          <div class="row">
            <span>{{ progress.name }}</span>
            <span>{{ progress.stateline }}</span>
          </div>
          <div class="bar"><i :style="{ width: progress.pct + '%' }"></i></div>
        </div>
        <div class="landed" v-if="landed">
          <div class="landed-title">{{ landed.title }}</div>
          <p v-if="landed.body">{{ landed.body }}</p>
          <pre v-if="landed.commands?.length" class="cmds">{{ landed.commands.join("\n") }}</pre>
        </div>
        <p class="hint">Click to pick files, or drop files/folders on this window. Then tap a device.</p>
      </aside>

      <section class="grid-wrap">
        <div class="section-label">2. Send to</div>
        <div v-if="!sendable.length && !blocked.length" class="empty">
          No devices yet. Open Tailscale on another machine.
        </div>
        <div class="grid">
          <button
            v-for="t in sendable"
            :key="t.stableID"
            class="card"
            :class="{ busy: sending && sendingTo === t.stableID }"
            :disabled="sending"
            @click="sendTo(t)"
          >
            <span class="avatar" :style="{ background: osColor(t.os) }">{{ osGlyph(t.os) }}</span>
            <span class="name">{{ t.hostname || t.dnsName }}</span>
            <span class="meta">{{ t.os }} · {{ t.online ? "online" : "offline" }}</span>
          </button>
          <div v-for="t in blocked" :key="t.stableID" class="card muted" :title="t.reason">
            <span class="avatar dim">{{ osGlyph(t.os) }}</span>
            <span class="name">{{ t.hostname || t.dnsName }}</span>
            <span class="meta">{{ t.reason || "not a Taildrop target" }}</span>
          </div>
        </div>
      </section>
    </main>

    <div class="drawer" v-if="inboxOpen">
      <div class="drawer-h">
        <strong>Inbox</strong>
        <button class="ghost" @click="inboxOpen = false">Close</button>
      </div>
      <p class="hint" v-if="platform !== 'linux'">
        On macOS and Windows, Tailscale usually saves to Downloads automatically.
        Use this if files are waiting in the daemon inbox.
      </p>
      <p v-if="!snap.inbox?.length" class="empty">Inbox empty.</p>
      <ul v-else class="inbox-list">
        <li v-for="f in snap.inbox" :key="f.name">{{ f.name }} <span class="muted">{{ fmtSize(f.size) }}</span></li>
      </ul>
      <div class="row-btns">
        <button class="primary" :disabled="!snap.inbox?.length || receiving" @click="receiveInbox">
          Save to {{ recvDirLabel }}
        </button>
        <button class="ghost" @click="pickRecvDir">Change folder</button>
      </div>
      <p class="hint" v-if="recvNote">{{ recvNote }}</p>
    </div>

    <div class="modal" v-if="pairOpen" @click.self="pairOpen = false">
      <div class="sheet">
        <div class="drawer-h">
          <strong>Pair phone</strong>
          <button class="ghost" @click="pairOpen = false">Close</button>
        </div>
        <p class="hint">Scan this from the Tailsend mobile app (Phase 2+), or copy the JSON.</p>
        <img v-if="qr" class="qr" :src="qr" alt="Pairing QR" />
        <pre class="json">{{ pairJson }}</pre>
        <button class="ghost" @click="copyJson">Copy JSON</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  Snapshot,
  SendTo,
  SelectFiles,
  SelectRecvDir,
  DefaultRecvDir,
  ReceiveTo,
  PairingJSON,
  PairingQR,
  Platform,
} from "../wailsjs/go/main/App";
import { EventsOn, OnFileDrop, OnFileDropOff } from "../wailsjs/runtime/runtime";

const snap = ref({ state: "Unknown", self: {}, targets: [], inbox: [] });
const files = ref([]);
const sending = ref(false);
const sendingTo = ref("");
const progress = ref(null);
const inboxOpen = ref(false);
const pairOpen = ref(false);
const pairJson = ref("");
const qr = ref("");
const recvDir = ref("");
const recvNote = ref("");
const receiving = ref(false);
const platform = ref("");
const landed = ref(null);

let timer = 0;
let offProgress = () => {};
let offDrop = () => {};

function byName(a, b) {
  const an = (a.hostname || a.dnsName || "").toLowerCase();
  const bn = (b.hostname || b.dnsName || "").toLowerCase();
  return an.localeCompare(bn);
}
const sendable = computed(() =>
  (snap.value.targets || []).filter((t) => !t.reason && t.online).slice().sort(byName)
);
const blocked = computed(() =>
  (snap.value.targets || []).filter((t) => t.reason || !t.online).slice().sort(byName)
);

const pillClass = computed(() => {
  const s = snap.value.state;
  if (s === "Running") return "ok";
  if (s === "NeedsLogin" || s === "Stopped") return "bad";
  return "mid";
});
const pillLabel = computed(() => snap.value.state || "Unknown");

const setupTitle = computed(() => {
  if (snap.value.state === "NeedsLogin") return "Tailscale needs login";
  if (snap.value.error && /not running/i.test(snap.value.error)) return "Tailscale is not running";
  return "";
});
const setupMessage = computed(() => {
  if (setupTitle.value) {
    return "Tailsend uses the official Tailscale app. Install it, sign in, then enable Send Files in the admin console.";
  }
  return "";
});

const recvDirLabel = computed(() => {
  if (!recvDir.value) return "Downloads";
  const parts = recvDir.value.split(/[/\\]/);
  return parts[parts.length - 1] || recvDir.value;
});

function osGlyph(os) {
  const s = (os || "").toLowerCase();
  if (s.includes("mac") || s === "darwin") return "";
  if (s.includes("win")) return "⊞";
  if (s.includes("android")) return "▶";
  if (s.includes("ios")) return "●";
  if (s.includes("linux")) return "◆";
  return "?";
}
function osColor(os) {
  const s = (os || "").toLowerCase();
  if (s.includes("mac") || s === "ios" || s === "darwin") return "#6b7280";
  if (s.includes("win")) return "#3b82f6";
  if (s.includes("android")) return "#22c55e";
  if (s.includes("linux")) return "#f59e0b";
  return "#5b8def";
}
function fmtSize(n) {
  if (n < 1024) return n + " B";
  const u = ["KiB", "MiB", "GiB"];
  let i = -1;
  let v = n;
  do {
    v /= 1024;
    i++;
  } while (v >= 1024 && i < u.length - 1);
  return v.toFixed(1) + " " + u[i];
}
function landingFor(t) {
  const os = (t.os || "").toLowerCase();
  const host = t.hostname || t.dnsName || "the device";
  if (os.includes("linux")) {
    return {
      title: `Sent to ${host}`,
      body: "Linux keeps Taildrop files in the daemon inbox. On that machine run:",
      commands: ["tailsend recv .", "sudo tailscale file get ."],
    };
  }
  if (os.includes("win")) {
    return {
      title: `Sent to ${host}`,
      body: "Windows Tailscale (v1.34+) usually saves to the signed-in user's Downloads folder, e.g. C:\\Users\\<you>\\Downloads. Older builds used Desktop.",
      commands: [],
    };
  }
  if (os.includes("mac") || os === "darwin") {
    return {
      title: `Sent to ${host}`,
      body: "macOS usually saves to ~/Downloads.",
      commands: [],
    };
  }
  if (os.includes("android")) {
    return {
      title: `Sent to ${host}`,
      body: "On Android, open the Tailscale notification, or look in Files → Downloads.",
      commands: [],
    };
  }
  if (os.includes("ios")) {
    return {
      title: `Sent to ${host}`,
      body: "On iOS, open the Tailscale notification to save the files in Files.app.",
      commands: [],
    };
  }
  return {
    title: `Sent to ${host}`,
    body: "Where it lands depends on that device's Tailscale app (often Downloads, or an inbox you drain with tailsend recv .).",
    commands: [],
  };
}

function basename(p) {
  return (p || "").split(/[/\\]/).pop();
}
function normalizePaths(arg, ...rest) {
  let list = [];
  if (Array.isArray(arg)) {
    list = arg;
  } else if (typeof arg === "string" && (arg.includes("\\") || arg.includes("/") || arg.length > 1)) {
    list = rest.length ? [arg, ...rest] : [arg];
  } else if (arg != null && typeof arg === "object" && typeof arg.length === "number") {
    list = Array.from(arg);
  }
  return list.filter((p) => typeof p === "string" && p.length > 1 && (p.includes("\\") || p.includes("/")));
}

function addPaths(...args) {
  for (const p of normalizePaths(...args)) {
    if (files.value.some((f) => f.path === p)) continue;
    files.value.push({ path: p, name: basename(p) });
  }
}

function urisToPaths(uriList) {
  const out = [];
  for (const line of String(uriList || "").split(/\r?\n/)) {
    const u = line.trim();
    if (!u || u.startsWith("#")) continue;
    if (!u.toLowerCase().startsWith("file:")) continue;
    let p = decodeURIComponent(u.replace(/^file:\/\/(localhost)?/i, ""));
    if (/^\/[A-Za-z]:\//.test(p)) p = p.slice(1).replace(/\//g, "\\");
    if (p) out.push(p);
  }
  return out;
}

function isFileDrag(dt) {
  if (!dt?.types) return false;
  const types = [...dt.types];
  return types.includes("Files") || types.includes("text/uri-list");
}

function onPageDragOver(e) {
  if (!isFileDrag(e.dataTransfer)) return;
  e.preventDefault();
  e.dataTransfer.dropEffect = "copy";
}

function onPageDrop(e) {
  if (!isFileDrag(e.dataTransfer)) return;
  e.preventDefault();
  e.stopPropagation();
  let uris = "";
  try {
    uris = e.dataTransfer.getData("text/uri-list");
  } catch (_) {}
  if (uris) addPaths(urisToPaths(uris));
}

async function refresh() {
  try {
    snap.value = await Snapshot();
  } catch (e) {
    snap.value = { state: "Unknown", error: String(e), targets: [], inbox: [] };
  }
}

async function pickFiles() {
  try {
    const picked = await SelectFiles();
    addPaths(picked);
    return picked || [];
  } catch (_) {
    return [];
  }
}

async function sendTo(t) {
  if (!files.value.length) {
    await pickFiles();
  }
  if (!files.value.length) return;
  sending.value = true;
  sendingTo.value = t.stableID;
  landed.value = null;
  progress.value = { name: "Starting…", pct: 0, stateline: "" };
  try {
    await SendTo(
      t.stableID,
      files.value.map((f) => f.path)
    );
    files.value = [];
    progress.value = { name: "Sent to " + (t.hostname || t.dnsName), pct: 100, stateline: "done" };
    landed.value = landingFor(t);
  } catch (e) {
    progress.value = { name: "Failed", pct: 0, stateline: String(e) };
  } finally {
    sending.value = false;
    sendingTo.value = "";
    refresh();
  }
}

async function openPair() {
  pairOpen.value = true;
  try {
    pairJson.value = await PairingJSON();
    qr.value = await PairingQR();
  } catch (e) {
    pairJson.value = String(e);
    qr.value = "";
  }
}

async function copyJson() {
  try {
    await navigator.clipboard.writeText(pairJson.value);
  } catch (_) {}
}

async function pickRecvDir() {
  try {
    const d = await SelectRecvDir();
    if (d) recvDir.value = d;
  } catch (_) {}
}

async function receiveInbox() {
  receiving.value = true;
  recvNote.value = "";
  try {
    const written = await ReceiveTo(recvDir.value || "", "rename");
    recvNote.value = written?.length ? "Saved " + written.length + " file(s)." : "Nothing to save.";
    await refresh();
  } catch (e) {
    recvNote.value = String(e);
  } finally {
    receiving.value = false;
  }
}

onMounted(async () => {
  try {
    platform.value = await Platform();
    recvDir.value = await DefaultRecvDir();
  } catch (_) {}
  await refresh();
  timer = window.setInterval(refresh, 15000);
  offProgress = EventsOn("progress", (p) => {
    const total = p.total > 0 ? p.total : 0;
    const pct = total ? Math.min(100, Math.round((p.sent / total) * 100)) : p.state === "done" ? 100 : 10;
    progress.value = {
      name: p.name || "",
      pct,
      stateline: p.state === "failed" ? p.err : p.state,
    };
  });
  offDrop = EventsOn("files-dropped", (...args) => addPaths(...args));
  OnFileDrop((_x, _y, paths) => addPaths(paths), false);
  window.addEventListener("dragover", onPageDragOver, true);
  window.addEventListener("drop", onPageDrop, true);
});

onUnmounted(() => {
  if (timer) clearInterval(timer);
  offProgress();
  offDrop();
  OnFileDropOff();
  window.removeEventListener("dragover", onPageDragOver, true);
  window.removeEventListener("drop", onPageDrop, true);
});
</script>

<style scoped>
.shell {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.top {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 20px 10px;
  border-bottom: 1px solid var(--line);
}
.shell.darwin .top {
  padding-left: 88px;
}
.brand {
  display: flex;
  gap: 10px;
  align-items: center;
  flex: 1;
}
.logo {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: var(--accent);
  color: #fff;
  display: grid;
  place-items: center;
  font-weight: 700;
}
.title {
  font-weight: 650;
  letter-spacing: -0.02em;
}
.sub {
  font-size: 12px;
  color: var(--muted);
}
.pill {
  font-size: 12px;
  padding: 4px 10px;
  border-radius: 999px;
  background: var(--bg-card);
  color: var(--muted);
}
.pill.ok {
  color: var(--good);
  background: #143326;
}
.pill.bad {
  color: var(--bad);
  background: #3a1c1c;
}
.pill.mid {
  color: var(--warn);
}
.actions {
  display: flex;
  gap: 8px;
}
.ghost,
.primary {
  border: 1px solid var(--line);
  background: var(--bg-card);
  color: var(--text);
  border-radius: 10px;
  padding: 6px 12px;
}
.primary {
  background: var(--accent);
  border-color: transparent;
  color: #fff;
}
.badge {
  margin-left: 6px;
  background: var(--accent);
  color: #fff;
  border-radius: 99px;
  padding: 0 6px;
  font-size: 11px;
}
.banner {
  margin: 12px 20px 0;
  padding: 12px 14px;
  border-radius: var(--radius);
  background: #3a1c1c;
  color: #ffd4d4;
}
.banner.warn {
  background: #3a3214;
  color: #ffe9a8;
}
.banner a {
  color: #fff;
}
.banner p {
  margin: 6px 0 8px;
  color: #f0c8c8;
}
.main {
  flex: 1;
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 18px;
  padding: 16px 20px 20px;
  min-height: 0;
}
.section-label {
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--muted);
  margin-bottom: 10px;
}
.grid-wrap {
  overflow: auto;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(148px, 1fr));
  gap: 10px;
}
.card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  text-align: left;
  background: var(--bg-card);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 14px;
  color: inherit;
}
.card:hover:not(:disabled) {
  background: var(--bg-card-hover);
  border-color: var(--accent);
}
.card.muted {
  opacity: 0.45;
}
.avatar {
  width: 36px;
  height: 36px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  color: #fff;
  font-size: 16px;
}
.avatar.dim {
  background: #3a4150;
}
.name {
  font-weight: 600;
  font-size: 14px;
}
.meta {
  font-size: 11px;
  color: var(--muted);
}
.side {
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.drop {
  --wails-drop-target: drop;
  flex: 1;
  border: 1px dashed var(--line);
  border-radius: var(--radius);
  background: var(--bg-raise);
  padding: 12px;
  color: var(--muted);
  font-size: 13px;
  overflow: auto;
  min-height: 140px;
}
.drop ul {
  list-style: none;
  margin: 0;
  padding: 0;
  color: var(--text);
}
.drop li {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 4px 0;
}
.x {
  border: 0;
  background: transparent;
  color: var(--muted);
}
.hint {
  font-size: 12px;
  color: var(--muted);
  margin: 10px 0 0;
}
.landed {
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #143326;
  color: #d8f5e6;
  font-size: 12px;
}
.landed-title {
  font-weight: 650;
  margin-bottom: 4px;
}
.landed p {
  margin: 0 0 8px;
  color: #b7e0c8;
}
.cmds {
  margin: 0;
  padding: 8px 10px;
  background: #0d0f14;
  border-radius: 8px;
  font-size: 12px;
  color: #e8eaed;
  overflow: auto;
}
.progress {
  margin-top: 12px;
}
.progress .row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--muted);
}
.bar {
  height: 6px;
  background: #2a303b;
  border-radius: 99px;
  margin-top: 6px;
  overflow: hidden;
}
.bar i {
  display: block;
  height: 100%;
  background: var(--accent);
}
.empty {
  color: var(--muted);
  font-size: 13px;
}
.drawer,
.sheet {
  background: var(--bg-raise);
  border-top: 1px solid var(--line);
  padding: 14px 20px 20px;
}
.drawer-h {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.inbox-list {
  padding-left: 18px;
}
.row-btns {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
.modal {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  display: grid;
  place-items: center;
  z-index: 20;
}
.sheet {
  width: min(440px, 92vw);
  border: 1px solid var(--line);
  border-radius: 16px;
  border-top: 1px solid var(--line);
}
.qr {
  display: block;
  width: 220px;
  height: 220px;
  margin: 12px auto;
  background: #fff;
  border-radius: 8px;
}
.json {
  font-size: 11px;
  background: #0d0f14;
  padding: 10px;
  border-radius: 8px;
  overflow: auto;
  max-height: 160px;
}
.muted {
  color: var(--muted);
}
@media (max-width: 800px) {
  .main {
    grid-template-columns: 1fr;
  }
}
</style>
