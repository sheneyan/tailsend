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
}
