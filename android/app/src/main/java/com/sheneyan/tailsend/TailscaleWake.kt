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
