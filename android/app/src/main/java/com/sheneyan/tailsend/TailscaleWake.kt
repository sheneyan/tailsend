package com.sheneyan.tailsend

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager

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
    private var skipNextProcessStop: Boolean = false

    fun onForeground() {
        val ips = try {
            wake.ips()
        } catch (_: Exception) {
            emptyList()
        }
        val installed = try {
            wake.installed()
        } catch (_: Exception) {
            false
        }
        phase = decideTailscalePrompt(ips, installed, declined, phase)
    }

    fun onProcessBackground() {
        if (skipNextProcessStop) {
            skipNextProcessStop = false
            return
        }
        declined = false
    }

    fun ignoreNextProcessStop() {
        skipNextProcessStop = true
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
            try {
                wake.sendConnect()
            } catch (_: Exception) {
            }
            val ok = waitForTailscaleIPs(wake.ips, waitAttempts, waitMs, sleep)
            phase = if (ok) TailscalePrompt.None else TailscalePrompt.Failed
        } catch (_: Exception) {
            phase = TailscalePrompt.Failed
        }
    }

    fun openApp(): Boolean = try {
        wake.openApp()
    } catch (_: Exception) {
        false
    }

    fun markMissing() {
        phase = TailscalePrompt.Missing
    }
}

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
            val i = app.packageManager.getLaunchIntentForPackage(TAILSCALE_PACKAGE)
            if (i == null) {
                false
            } else {
                i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                app.startActivity(i)
                true
            }
        },
    )
}
