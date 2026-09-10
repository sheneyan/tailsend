package com.sheneyan.tailsend

import org.json.JSONArray

/**
 * Parses `tailsend pair` / GUI Pair phone JSON: an array of Target objects.
 * Entries without peerAPIURL cannot be sent to and are dropped.
 */
fun parsePairingJson(raw: String): List<Target> {
    val trimmed = raw.trim()
    if (trimmed.isEmpty()) {
        throw IllegalArgumentException("empty pairing JSON")
    }
    if (!trimmed.startsWith("[")) {
        throw IllegalArgumentException("pairing JSON must be a Target array")
    }
    val array = JSONArray(trimmed)
    val out = ArrayList<Target>(array.length())
    val seen = HashSet<String>()
    for (i in 0 until array.length()) {
        val o = array.getJSONObject(i)
        val url = o.optString("peerAPIURL").trim()
        if (url.isEmpty()) continue
        val id = o.optString("stableID")
        if (id.isNotEmpty() && !seen.add(id)) continue
        val ips = ArrayList<String>()
        val ipArr = o.optJSONArray("ips")
        if (ipArr != null) {
            for (j in 0 until ipArr.length()) {
                val ip = ipArr.optString(j)
                if (ip.isNotEmpty()) ips.add(ip)
            }
        }
        out.add(
            Target(
                stableID = id,
                hostname = o.optString("hostname"),
                dnsName = o.optString("dnsName"),
                ips = ips,
                os = o.optString("os"),
                online = o.optBoolean("online", true),
                peerAPIURL = url,
                reason = o.optString("reason"),
            ),
        )
    }
    if (out.isEmpty()) {
        throw IllegalArgumentException("no devices with peerAPIURL in pairing JSON")
    }
    return out
}

fun pathEncode(name: String): String {
    val bytes = name.toByteArray(Charsets.UTF_8)
    val sb = StringBuilder(bytes.size * 3)
    for (b in bytes) {
        val u = b.toInt() and 0xFF
        val unreserved = (u >= '0'.code && u <= '9'.code) ||
            (u >= 'A'.code && u <= 'Z'.code) ||
            (u >= 'a'.code && u <= 'z'.code) ||
            u == '-'.code || u == '.'.code || u == '_'.code || u == '~'.code
        if (unreserved) {
            sb.append(u.toChar())
        } else {
            sb.append('%')
            sb.append("0123456789ABCDEF"[u shr 4])
            sb.append("0123456789ABCDEF"[u and 0x0F])
        }
    }
    return sb.toString()
}

fun putUrl(peerAPIURL: String, filename: String): String {
    val base = peerAPIURL.trim().trimEnd('/')
    return "$base/v0/put/${pathEncode(filename)}"
}
