package com.sheneyan.tailsend

data class Target(
    val stableID: String,
    val hostname: String,
    val dnsName: String,
    val ips: List<String>,
    val os: String,
    val online: Boolean,
    val peerAPIURL: String,
    val reason: String = "",
) {
    val label: String
        get() = hostname.ifBlank { dnsName.ifBlank { stableID } }
}
