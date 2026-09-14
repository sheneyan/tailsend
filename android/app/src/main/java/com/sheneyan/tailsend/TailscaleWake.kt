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
