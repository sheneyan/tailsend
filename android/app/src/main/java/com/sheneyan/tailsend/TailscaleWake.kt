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
