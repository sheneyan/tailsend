package com.sheneyan.tailsend

import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.io.InputStream
import java.net.Inet4Address
import java.net.Inet6Address
import java.net.InetAddress
import java.net.NetworkInterface
import java.net.UnknownHostException
import java.util.zip.CRC32

const val LIST_PLEASE_NAME = "tailsend-list-please"
const val PAIR_JSON_NAME = "tailsend-pair.json"

fun isPairJsonFilename(name: String): Boolean {
    val base = name.substringAfterLast('/').substringAfterLast('\\')
    return base.equals(PAIR_JSON_NAME, ignoreCase = true)
}

fun peerAPIPorts(ip: String): List<Int> = peerAPIPorts(InetAddress.getByName(ip))

fun peerAPIPorts(ip: InetAddress): List<Int> {
    val a16 = ipTo16(ip)
    val out = ArrayList<Int>(5)
    for (tryN in 0 until 5) {
        val hash = byteArrayOf(a16[13], a16[14], a16[15])
        hash[0] = (hash[0] + tryN).toByte()
        val crc = CRC32()
        crc.update(hash)
        out.add((32 shl 10) or (crc.value.toInt() and 0xFFFF))
    }
    return out
}

internal fun ipTo16(ip: InetAddress): ByteArray {
    val a = ip.address
    if (a.size == 16) return a
    val out = ByteArray(16)
    out[10] = 0xff.toByte()
    out[11] = 0xff.toByte()
    System.arraycopy(a, 0, out, 12, 4)
    return out
}

fun listPleaseJSON(from: List<String>): String {
    val arr = JSONArray()
    for (ip in from) {
        arr.put(ip)
    }
    return JSONObject().put("v", 1).put("from", arr).toString()
}

fun hashedPeerAPIURLs(ip: InetAddress): List<String> {
    val host = if (ip is Inet6Address) {
        val raw = ip.hostAddress?.substringBefore('%') ?: return emptyList()
        "[$raw]"
    } else {
        ip.hostAddress ?: return emptyList()
    }
    return peerAPIPorts(ip).map { "http://$host:$it" }
}

fun requestListRefresh(
    hostname: String,
    resolve: (String) -> Array<InetAddress> = { InetAddress.getAllByName(it) },
    localIPs: () -> List<String> = { tailscaleIPs() },
    put: (peerAPIURL: String, filename: String, size: Long, body: InputStream) -> Long = { url, name, size, body ->
        putFile(url, name, size, body)
    },
) {
    val host = hostname.trim().trimEnd('.')
    if (host.isEmpty()) {
        throw SendFailedException("Type a computer hostname, or scan QR.")
    }
    val from = localIPs().filter { it.isNotBlank() }
    if (from.isEmpty()) {
        throw SendFailedException("No Tailscale address on this phone. Open official Tailscale, or scan QR.")
    }
    val addrs = try {
        resolve(host)
    } catch (e: UnknownHostException) {
        throw SendFailedException("Cannot resolve $host. Turn on Tailscale DNS, or scan QR.", e)
    }
    if (addrs.isEmpty()) {
        throw SendFailedException("Cannot resolve $host. Turn on Tailscale DNS, or scan QR.")
    }
    val payload = listPleaseJSON(from).toByteArray(Charsets.UTF_8)
    var last: Exception? = null
    val seen = HashSet<String>()
    for (addr in addrs) {
        for (url in hashedPeerAPIURLs(addr)) {
            if (!seen.add(url)) continue
            try {
                put(url, LIST_PLEASE_NAME, payload.size.toLong(), ByteArrayInputStream(payload))
                return
            } catch (e: Exception) {
                last = e
            }
        }
    }
    throw SendFailedException(
        "Cannot reach $host. Open official Tailscale, or scan QR.",
        last,
    )
}

fun tailscaleIPs(
    nics: List<NetworkInterface> = NetworkInterface.getNetworkInterfaces()?.toList().orEmpty(),
): List<String> {
    val out = ArrayList<String>()
    for (nic in nics) {
        if (!nic.isUp) continue
        val addrs = nic.inetAddresses ?: continue
        for (addr in addrs) {
            if (!isTailscaleAddr(addr)) continue
            val s = addr.hostAddress?.substringBefore('%') ?: continue
            out.add(s)
        }
    }
    return out.distinct()
}

fun isTailscaleAddr(addr: InetAddress): Boolean {
    val a = addr.address ?: return false
    if (addr is Inet4Address && a.size == 4) {
        val b0 = a[0].toInt() and 0xFF
        val b1 = a[1].toInt() and 0xFF
        return b0 == 100 && (b1 and 0xC0) == 64
    }
    if (addr is Inet6Address && a.size == 16) {
        return a[0] == 0xfd.toByte() && a[1] == 0x7a.toByte() &&
            a[2] == 0x11.toByte() && a[3] == 0x5c.toByte() &&
            a[4] == 0xa1.toByte() && a[5] == 0xe0.toByte()
    }
    return false
}
