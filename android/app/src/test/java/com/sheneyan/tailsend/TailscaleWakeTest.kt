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
}
