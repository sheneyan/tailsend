# Android Wake Tailscale Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When the Android sender comes to the foreground without a Tailscale address, show an in-layout prompt and, if the user taps Start, send official `CONNECT_VPN` without leaving Tailsend.

**Architecture:** Pure Kotlin prompt state (`TailscalePrompt`, `decideTailscalePrompt`, `waitForTailscaleIPs`, `TailscalePromptController`) is unit-tested on the JVM with a fake `TailscaleWake`. `SendViewModel` holds one controller, copies `phase` into `SendUiState`, and runs `start()` on `Dispatchers.IO`. `MainActivity.onResume` calls `onForeground()`. The UI is an inline card (not a modal `Dialog`) so Pair / pick / send stay tappable. Success is inferred only from `tailscaleIPs()`.

**Tech Stack:** Kotlin, JUnit 4, AndroidX Compose Material3, `lifecycle-process` 2.8.7, official Tailscale package `com.tailscale.ipn`.

**Spec:** `docs/superpowers/specs/2026-09-14-android-wake-tailscale-design.md`

---

## File map

| File | Role |
|---|---|
| Create `android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt` | Constants, `TailscalePrompt`, `TailscaleWake`, `decideTailscalePrompt`, `waitForTailscaleIPs`, `TailscalePromptController`, `tailscaleWake(Context)` |
| Create `android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt` | JVM tests for decide / wait / controller (no real broadcast) |
| Modify `android/app/src/main/java/com/sheneyan/tailsend/SendViewModel.kt` | `tailscalePrompt` on `SendUiState`; wire controller; `onForeground` / `declineTailscale` / `startTailscale` / `openTailscale` / `onProcessBackground` |
| Modify `android/app/src/main/java/com/sheneyan/tailsend/ui/TailsendScreen.kt` | Inline prompt card + Start / Not now / Open Tailscale callbacks |
| Modify `android/app/src/main/java/com/sheneyan/tailsend/MainActivity.kt` | `onResume` → `vm.onForeground()`; pass new callbacks |
| Modify `android/app/src/main/AndroidManifest.xml` | `<queries>` for `com.tailscale.ipn` |
| Modify `android/app/build.gradle.kts` | `androidx.lifecycle:lifecycle-process:2.8.7` |
| Modify `android/README.md`, `CHANGELOG.md` | User-facing note |

Do not treat `TRANSPORT_VPN` as connected. Do not `startActivity` on Tailscale except **Open Tailscale**. Do not add Robolectric.

---

### Task 1: `decideTailscalePrompt`

**Files:**
- Create: `android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt`
- Create: `android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt`

- [ ] **Step 1: Write the failing tests**

Create `android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt`:

```kotlin
package com.sheneyan.tailsend

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TailscaleWakeTest {
    @Test
    fun emptyIpsInstalledNotDeclinedAsksConfirm() {
        assertEquals(
            TailscalePrompt.Confirm,
            decideTailscalePrompt(
                ips = emptyList(),
                installed = true,
                declinedThisSession = false,
                current = TailscalePrompt.None,
            ),
        )
    }

    @Test
    fun ipv4MeansNoPrompt() {
        assertEquals(
            TailscalePrompt.None,
            decideTailscalePrompt(
                ips = listOf("100.64.0.1"),
                installed = true,
                declinedThisSession = false,
                current = TailscalePrompt.Confirm,
            ),
        )
    }

    @Test
    fun ipv6MeansNoPrompt() {
        assertEquals(
            TailscalePrompt.None,
            decideTailscalePrompt(
                ips = listOf("fd7a:115c:a1e0::1"),
                installed = true,
                declinedThisSession = false,
                current = TailscalePrompt.Failed,
            ),
        )
    }

    @Test
    fun declinedThisSessionNoPrompt() {
        assertEquals(
            TailscalePrompt.None,
            decideTailscalePrompt(
                ips = emptyList(),
                installed = true,
                declinedThisSession = true,
                current = TailscalePrompt.None,
            ),
        )
    }

    @Test
    fun packageMissingIsMissingPhase() {
        assertEquals(
            TailscalePrompt.Missing,
            decideTailscalePrompt(
                ips = emptyList(),
                installed = false,
                declinedThisSession = false,
                current = TailscalePrompt.None,
            ),
        )
    }

    @Test
    fun startingIsNotInterrupted() {
        assertEquals(
            TailscalePrompt.Starting,
            decideTailscalePrompt(
                ips = emptyList(),
                installed = true,
                declinedThisSession = false,
                current = TailscalePrompt.Starting,
            ),
        )
    }

    @Test
    fun failedSticksUntilIpsAppear() {
        assertEquals(
            TailscalePrompt.Failed,
            decideTailscalePrompt(
                ips = emptyList(),
                installed = true,
                declinedThisSession = false,
                current = TailscalePrompt.Failed,
            ),
        )
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
export ANDROID_HOME="${ANDROID_HOME:-/Users/sheneyan/Documents/Work/Sources/AI/android-sdk}"
cd android && ./gradlew :app:testDebugUnitTest --tests com.sheneyan.tailsend.TailscaleWakeTest
```

Expected: FAIL compiling (`Unresolved reference: TailscalePrompt` / `decideTailscalePrompt`).

- [ ] **Step 3: Minimal implementation**

Create `android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt`:

```kotlin
package com.sheneyan.tailsend

enum class TailscalePrompt {
    None,
    Confirm,
    Starting,
    Failed,
    Missing,
}

fun decideTailscalePrompt(
    ips: List<String>,
    installed: Boolean,
    declinedThisSession: Boolean,
    current: TailscalePrompt,
): TailscalePrompt {
    if (ips.isNotEmpty()) return TailscalePrompt.None
    if (current == TailscalePrompt.Starting) return TailscalePrompt.Starting
    if (declinedThisSession) return TailscalePrompt.None
    if (!installed) return TailscalePrompt.Missing
    if (current == TailscalePrompt.Failed) return TailscalePrompt.Failed
    return TailscalePrompt.Confirm
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run the same `./gradlew :app:testDebugUnitTest --tests com.sheneyan.tailsend.TailscaleWakeTest`

Expected: `BUILD SUCCESSFUL`, 7 tests passed.

- [ ] **Step 5: Commit**

```bash
git add android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt \
  android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt
git commit -m "feat: decide when Android should prompt to start Tailscale"
```

---

### Task 2: `waitForTailscaleIPs`

**Files:**
- Modify: `android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt`
- Modify: `android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt`

- [ ] **Step 1: Write the failing tests**

Append to `TailscaleWakeTest`:

```kotlin
    @Test
    fun waitReturnsTrueWhenIpsAppear() {
        var n = 0
        val ok = waitForTailscaleIPs(
            ips = {
                n++
                if (n >= 3) listOf("100.64.0.1") else emptyList()
            },
            attempts = 16,
            delayMs = 500,
            sleep = {},
        )
        assertTrue(ok)
        assertEquals(3, n)
    }

    @Test
    fun waitReturnsFalseWhenIpsStayEmpty() {
        var sleeps = 0
        val ok = waitForTailscaleIPs(
            ips = { emptyList() },
            attempts = 16,
            delayMs = 500,
            sleep = { sleeps++ },
        )
        assertFalse(ok)
        assertEquals(16, sleeps)
    }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd android && ./gradlew :app:testDebugUnitTest --tests com.sheneyan.tailsend.TailscaleWakeTest`

Expected: FAIL compiling (`Unresolved reference: waitForTailscaleIPs`).

- [ ] **Step 3: Minimal implementation**

Append to `TailscaleWake.kt`:

```kotlin
const val TAILSCALE_WAIT_ATTEMPTS = 16
const val TAILSCALE_WAIT_MS = 500L

fun waitForTailscaleIPs(
    ips: () -> List<String>,
    attempts: Int = TAILSCALE_WAIT_ATTEMPTS,
    delayMs: Long = TAILSCALE_WAIT_MS,
    sleep: (Long) -> Unit = { Thread.sleep(it) },
): Boolean {
    repeat(attempts) {
        if (ips().isNotEmpty()) return true
        if (delayMs > 0) sleep(delayMs)
    }
    return ips().isNotEmpty()
}
```

16 × 500ms = 8 seconds when empty. Tests pass `sleep = {}` / count-only sleep so CI does not wait 8s.

- [ ] **Step 4: Run tests to verify they pass**

Run the same Gradle test command.

Expected: `BUILD SUCCESSFUL`, 9 tests passed.

- [ ] **Step 5: Commit**

```bash
git add android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt \
  android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt
git commit -m "feat: poll for Tailscale addresses after CONNECT_VPN"
```

---

### Task 3: `TailscalePromptController` (no real broadcast)

**Files:**
- Modify: `android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt`
- Modify: `android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt`

- [ ] **Step 1: Write the failing tests**

Append to `TailscaleWakeTest`:

```kotlin
    private fun fakeWake(
        ips: () -> List<String> = { emptyList() },
        installed: () -> Boolean = { true },
        sendConnect: () -> Unit = {},
        openApp: () -> Boolean = { true },
    ) = TailscaleWake(ips, installed, sendConnect, openApp)

    @Test
    fun foregroundConfirmWhenDownAndInstalled() {
        val c = TailscalePromptController(fakeWake())
        c.onForeground()
        assertEquals(TailscalePrompt.Confirm, c.phase)
    }

    @Test
    fun declineSuppressesUntilProcessBackground() {
        val c = TailscalePromptController(fakeWake())
        c.onForeground()
        c.decline()
        assertEquals(TailscalePrompt.None, c.phase)
        c.onForeground()
        assertEquals(TailscalePrompt.None, c.phase)
        c.onProcessBackground()
        c.onForeground()
        assertEquals(TailscalePrompt.Confirm, c.phase)
    }

    @Test
    fun startDoesNotBroadcastWhenMissing() {
        var sent = 0
        val c = TailscalePromptController(
            fakeWake(installed = { false }, sendConnect = { sent++ }),
        )
        c.start()
        assertEquals(0, sent)
        assertEquals(TailscalePrompt.Missing, c.phase)
    }

    @Test
    fun startBroadcastsThenNoneWhenIpsAppear() {
        var sent = 0
        var n = 0
        val c = TailscalePromptController(
            fakeWake(
                ips = {
                    n++
                    if (sent > 0 && n >= 2) listOf("100.64.0.1") else emptyList()
                },
                sendConnect = { sent++ },
            ),
        )
        c.start(waitAttempts = 8, waitMs = 1, sleep = {})
        assertEquals(1, sent)
        assertEquals(TailscalePrompt.None, c.phase)
    }

    @Test
    fun startBroadcastsThenFailedWhenIpsStayEmpty() {
        var sent = 0
        val c = TailscalePromptController(
            fakeWake(
                ips = { emptyList() },
                sendConnect = { sent++ },
            ),
        )
        c.start(waitAttempts = 3, waitMs = 1, sleep = {})
        assertEquals(1, sent)
        assertEquals(TailscalePrompt.Failed, c.phase)
    }

    @Test
    fun startTreatsSendExceptionAsFailedIfNoIps() {
        val c = TailscalePromptController(
            fakeWake(
                ips = { emptyList() },
                sendConnect = { error("broadcast") },
            ),
        )
        c.start(waitAttempts = 2, waitMs = 1, sleep = {})
        assertEquals(TailscalePrompt.Failed, c.phase)
    }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd android && ./gradlew :app:testDebugUnitTest --tests com.sheneyan.tailsend.TailscaleWakeTest`

Expected: FAIL compiling (`Unresolved reference: TailscaleWake` / `TailscalePromptController`).

- [ ] **Step 3: Minimal implementation**

Append to `TailscaleWake.kt`:

```kotlin
data class TailscaleWake(
    val ips: () -> List<String>,
    val installed: () -> Boolean,
    val sendConnect: () -> Unit,
    val openApp: () -> Boolean,
)

class TailscalePromptController(
    private val wake: TailscaleWake,
) {
    var phase: TailscalePrompt = TailscalePrompt.None
        private set
    var declined: Boolean = false
        private set

    fun onForeground() {
        phase = decideTailscalePrompt(wake.ips(), wake.installed(), declined, phase)
    }

    fun onProcessBackground() {
        declined = false
    }

    fun decline() {
        declined = true
        phase = TailscalePrompt.None
    }

    fun start(
        waitAttempts: Int = TAILSCALE_WAIT_ATTEMPTS,
        waitMs: Long = TAILSCALE_WAIT_MS,
        sleep: (Long) -> Unit = { Thread.sleep(it) },
    ) {
        if (!wake.installed()) {
            phase = TailscalePrompt.Missing
            return
        }
        phase = TailscalePrompt.Starting
        try {
            wake.sendConnect()
        } catch (_: Exception) {
        }
        val ok = waitForTailscaleIPs(wake.ips, waitAttempts, waitMs, sleep)
        phase = if (ok) TailscalePrompt.None else TailscalePrompt.Failed
    }

    fun openApp(): Boolean = try {
        wake.openApp()
    } catch (_: Exception) {
        false
    }
}
```

`start()` is blocking; the ViewModel (Task 4) must call it on `Dispatchers.IO`. Tests use `waitAttempts = 3` and `sleep = {}` so they do not sleep 8s.

- [ ] **Step 4: Run tests to verify they pass**

Run the same Gradle test command.

Expected: `BUILD SUCCESSFUL`, all `TailscaleWakeTest` tests passed.

- [ ] **Step 5: Commit**

```bash
git add android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt \
  android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt
git commit -m "feat: Tailscale prompt controller with fake wake"
```

---

### Task 4: Context factory + ViewModel wire-up

**Files:**
- Modify: `android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt`
- Modify: `android/app/src/main/java/com/sheneyan/tailsend/SendViewModel.kt`
- Modify: `android/app/build.gradle.kts`

- [ ] **Step 1: Add `lifecycle-process`**

In `android/app/build.gradle.kts`, next to the other lifecycle lines:

```kotlin
    implementation("androidx.lifecycle:lifecycle-process:2.8.7")
```

- [ ] **Step 2: Real `tailscaleWake(Context)`**

Add these imports at the top of `TailscaleWake.kt` (after `package`, before `enum class`):

```kotlin
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
```

Append to the same file:

```kotlin
const val TAILSCALE_PACKAGE = "com.tailscale.ipn"
const val TAILSCALE_RECEIVER = "com.tailscale.ipn.IPNReceiver"
const val INTENT_CONNECT_VPN = "com.tailscale.ipn.CONNECT_VPN"

fun tailscaleWake(context: Context): TailscaleWake {
    val app = context.applicationContext
    return TailscaleWake(
        ips = { tailscaleIPs() },
        installed = {
            try {
                @Suppress("DEPRECATION")
                app.packageManager.getPackageInfo(TAILSCALE_PACKAGE, 0)
                true
            } catch (_: PackageManager.NameNotFoundException) {
                false
            }
        },
        sendConnect = {
            val i = Intent(INTENT_CONNECT_VPN).setClassName(TAILSCALE_PACKAGE, TAILSCALE_RECEIVER)
            app.sendBroadcast(i)
        },
        openApp = {
            val i = app.packageManager.getLaunchIntentForPackage(TAILSCALE_PACKAGE) ?: return@tailscaleWake false
            i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            app.startActivity(i)
            true
        },
    )
}
```

- [ ] **Step 3: Extend `SendUiState` and `SendViewModel`**

In `SendViewModel.kt`:

Add imports:

```kotlin
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.ProcessLifecycleOwner
```

Add field on `SendUiState` (keep other fields unchanged):

```kotlin
    val tailscalePrompt: TailscalePrompt = TailscalePrompt.None,
```

Inside `SendViewModel`, add properties and a second `init` (existing pairing `init` stays as-is):

```kotlin
    private val wake = tailscaleWake(app)
    private val tailscale = TailscalePromptController(wake)
    private val processObserver = object : DefaultLifecycleObserver {
        override fun onStop(owner: LifecycleOwner) {
            onProcessBackground()
        }
    }

    init {
        ProcessLifecycleOwner.get().lifecycle.addObserver(processObserver)
    }

    override fun onCleared() {
        ProcessLifecycleOwner.get().lifecycle.removeObserver(processObserver)
        super.onCleared()
    }
```

Add methods:

```kotlin
    fun onForeground() {
        tailscale.onForeground()
        publishPrompt()
    }

    fun onProcessBackground() {
        tailscale.onProcessBackground()
    }

    fun declineTailscale() {
        tailscale.decline()
        publishPrompt()
    }

    fun startTailscale() {
        viewModelScope.launch {
            _state.update { it.copy(tailscalePrompt = TailscalePrompt.Starting) }
            withContext(Dispatchers.IO) { tailscale.start() }
            publishPrompt()
        }
    }

    fun openTailscale() {
        viewModelScope.launch {
            val ok = withContext(Dispatchers.IO) { tailscale.openApp() }
            if (!ok) {
                tailscale.markMissing()
            }
            publishPrompt()
        }
    }

    private fun publishPrompt() {
        _state.update { it.copy(tailscalePrompt = tailscale.phase) }
    }
```

`openTailscale` needs `markMissing()` if launch fails. Add to `TailscalePromptController` in `TailscaleWake.kt`:

```kotlin
    fun markMissing() {
        phase = TailscalePrompt.Missing
    }
```

Add a unit test in `TailscaleWakeTest.kt`:

```kotlin
    @Test
    fun markMissingSetsPhase() {
        val c = TailscalePromptController(fakeWake())
        c.markMissing()
        assertEquals(TailscalePrompt.Missing, c.phase)
    }
```

If `openApp` returns false, do **not** send `CONNECT_VPN`.

Do not reset `declined` on Activity `onStop` from the file picker via Activity lifecycle; only `ProcessLifecycleOwner.onStop` (already in the observer).

- [ ] **Step 4: Run unit tests**

```bash
export ANDROID_HOME="${ANDROID_HOME:-/Users/sheneyan/Documents/Work/Sources/AI/android-sdk}"
cd android && ./gradlew :app:testDebugUnitTest
```

Expected: `BUILD SUCCESSFUL`, existing tests + `TailscaleWakeTest` all pass.

- [ ] **Step 5: Commit**

```bash
git add android/app/build.gradle.kts \
  android/app/src/main/java/com/sheneyan/tailsend/TailscaleWake.kt \
  android/app/src/main/java/com/sheneyan/tailsend/SendViewModel.kt \
  android/app/src/test/java/com/sheneyan/tailsend/TailscaleWakeTest.kt
git commit -m "feat: wire Tailscale prompt into the Android send ViewModel"
```

---

### Task 5: Inline prompt card

**Files:**
- Modify: `android/app/src/main/java/com/sheneyan/tailsend/ui/TailsendScreen.kt`
- Modify: `android/app/src/main/java/com/sheneyan/tailsend/MainActivity.kt`

- [ ] **Step 1: Extend `TailsendScreen` parameters**

Add after `onCloseScan`:

```kotlin
    onStartTailscale: () -> Unit,
    onDeclineTailscale: () -> Unit,
    onOpenTailscale: () -> Unit,
```

- [ ] **Step 2: Inline card (not a modal Dialog)**

Insert in the main `Column`, after the subtitle `Text("Parasite sender...")` and before the file-picker `Box`. Pair / devices / pick stay clickable.

```kotlin
            if (state.tailscalePrompt != TailscalePrompt.None) {
                Surface(
                    shape = RoundedCornerShape(12.dp),
                    color = Card,
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 16.dp),
                ) {
                    Column(Modifier.padding(16.dp)) {
                        Text(
                            when (state.tailscalePrompt) {
                                TailscalePrompt.Confirm -> "Tailscale is not connected. Start it?"
                                TailscalePrompt.Starting -> "Starting Tailscale…"
                                TailscalePrompt.Failed ->
                                    "Could not start Tailscale. It may need login, VPN permission, or another VPN is active."
                                TailscalePrompt.Missing ->
                                    "Install official Tailscale and sign in. Tailsend does not embed a node."
                                TailscalePrompt.None -> ""
                            },
                            color = Color.White,
                            fontSize = 14.sp,
                        )
                        Row(
                            modifier = Modifier.padding(top = 8.dp),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            when (state.tailscalePrompt) {
                                TailscalePrompt.Confirm -> {
                                    Button(onClick = onStartTailscale) { Text("Start") }
                                    TextButton(onClick = onDeclineTailscale) { Text("Not now") }
                                }
                                TailscalePrompt.Failed -> {
                                    Button(onClick = onOpenTailscale) { Text("Open Tailscale") }
                                    TextButton(onClick = onDeclineTailscale) { Text("Not now") }
                                }
                                TailscalePrompt.Missing -> {
                                    TextButton(onClick = onDeclineTailscale) { Text("Not now") }
                                }
                                else -> {}
                            }
                        }
                    }
                }
            }
```

Add import: `import com.sheneyan.tailsend.TailscalePrompt`

Use the existing `Button` / `TextButton` already imported. Do **not** wrap this in `Dialog`.

- [ ] **Step 3: Pass callbacks from `MainActivity`**

In `setContent` `TailsendScreen(...)` add:

```kotlin
                    onStartTailscale = vm::startTailscale,
                    onDeclineTailscale = vm::declineTailscale,
                    onOpenTailscale = vm::openTailscale,
```

And override `onResume`:

```kotlin
    override fun onResume() {
        super.onResume()
        vm.onForeground()
    }
```

- [ ] **Step 4: Manifest `<queries>`**

In `android/app/src/main/AndroidManifest.xml`, inside `<manifest>` (sibling of `<application>`), add:

```xml
    <queries>
        <package android:name="com.tailscale.ipn" />
    </queries>
```

- [ ] **Step 5: Compile and unit tests**

```bash
export ANDROID_HOME="${ANDROID_HOME:-/Users/sheneyan/Documents/Work/Sources/AI/android-sdk}"
cd android && ./gradlew :app:testDebugUnitTest :app:compileDebugKotlin
```

Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 6: Commit**

```bash
git add android/app/src/main/java/com/sheneyan/tailsend/ui/TailsendScreen.kt \
  android/app/src/main/java/com/sheneyan/tailsend/MainActivity.kt \
  android/app/src/main/AndroidManifest.xml
git commit -m "feat: prompt in Tailsend to start official Tailscale"
```

---

### Task 6: Docs

**Files:**
- Modify: `android/README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: README**

After the first paragraph of `android/README.md` (“Sender only…”), add:

```markdown
On open (including share-sheet entry), if this phone has no Tailscale
address (`100.64/10` or `fd7a:115c:a1e0::/48`), Tailsend asks to start
official Tailscale and sends `com.tailscale.ipn.CONNECT_VPN`. That does
not open the Tailscale UI. First-time login and the VPN permission
dialog still happen in the official app. Another VPN occupying Android’s
slot looks like a failed start; use **Open Tailscale**.
```

- [ ] **Step 2: CHANGELOG**

Under `### Phase 2 — Android sender` add:

```markdown
- On foreground, if there is no Tailscale address, ask to Start official
  Tailscale via `CONNECT_VPN` (stay in Tailsend). Open Tailscale UI is
  the fallback.
```

- [ ] **Step 3: Commit**

```bash
git add android/README.md CHANGELOG.md
git commit -m "docs: Android may ask to start official Tailscale"
```

---

### Task 7: Full test run + manual script

- [ ] **Step 1: Run all Android unit tests**

```bash
export ANDROID_HOME="${ANDROID_HOME:-/Users/sheneyan/Documents/Work/Sources/AI/android-sdk}"
cd android && ./gradlew :app:testDebugUnitTest
```

Expected: `BUILD SUCCESSFUL`, 0 failed.

- [ ] **Step 2: Manual check (device with official Tailscale)**

1. Tailscale logged in, VPN off, open Tailsend → card **Tailscale is not connected. Start it?** → **Start** → VPN comes up, card goes away, send still works.
2. **Not now** → no card until Tailsend is backgrounded (leave the app) and resumed.
3. Uninstall Tailscale → **Install official Tailscale…**, no `CONNECT_VPN`.
4. Another VPN on, Tailscale off → **Start** → timeout copy → **Open Tailscale**.
5. Share a photo into Tailsend with Tailscale off → same card above the send tray; Pair and devices still tappable.

Do not claim this task done without either performing those steps or stating they were not run.

---

## Spec coverage

| Spec item | Task |
|---|---|
| Detect via `tailscaleIPs` / `100.64/10` + `fd7a:115c:a1e0::/48`, not `TRANSPORT_VPN` | 1, 4 (`wake.ips = { tailscaleIPs() }`) |
| `onResume` check | 5 `MainActivity.onResume` |
| Confirm / Starting / timeout / missing copy (English) | 5 |
| Start → explicit `CONNECT_VPN` to `IPNReceiver`, stay in Tailsend | 3, 4 |
| Poll 500ms × 16 = 8s | 2 |
| Open Tailscale only on timeout button | 4, 5 |
| Missing package: no Start, no broadcast | 1, 3 |
| Not now until process background (`ProcessLifecycleOwner`) | 3, 4 |
| Inline, send/Pair/pick still enabled | 5 (card, not `Dialog`) |
| `<queries>` for `com.tailscale.ipn` | 5 |
| JVM tests, no live `CONNECT_VPN` | 1–3 |
| README + CHANGELOG | 6 |
| Manual paths | 7 |
