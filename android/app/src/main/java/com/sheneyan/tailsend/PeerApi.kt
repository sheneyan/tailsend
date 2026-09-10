package com.sheneyan.tailsend

import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL

class SendFailedException(message: String, cause: Throwable? = null) : IOException(message, cause)

fun putFile(
    peerAPIURL: String,
    filename: String,
    size: Long,
    body: InputStream,
    onProgress: (sent: Long, total: Long) -> Unit = { _, _ -> },
): Long {
    val dest = putUrl(peerAPIURL, filename)
    val conn = (URL(dest).openConnection() as HttpURLConnection).apply {
        requestMethod = "PUT"
        doOutput = true
        useCaches = false
        connectTimeout = 15_000
        readTimeout = 0
        setRequestProperty("Content-Type", "application/octet-stream")
        if (size >= 0) {
            setRequestProperty("Content-Length", size.toString())
            setFixedLengthStreamingMode(size)
        } else {
            setChunkedStreamingMode(64 * 1024)
        }
    }
    try {
        conn.outputStream.use { out ->
            val buf = ByteArray(64 * 1024)
            var sent = 0L
            while (true) {
                val n = body.read(buf)
                if (n < 0) break
                out.write(buf, 0, n)
                sent += n
                onProgress(sent, size)
            }
            out.flush()
        }
        val code = conn.responseCode
        if (code !in 200..299) {
            val err = (conn.errorStream ?: conn.inputStream)?.bufferedReader()?.readText().orEmpty()
            throw SendFailedException(httpError(code, err))
        }
        return if (size >= 0) size else conn.contentLengthLong.coerceAtLeast(0)
    } catch (e: SendFailedException) {
        throw e
    } catch (e: java.net.ConnectException) {
        throw SendFailedException(
            "Cannot reach ${targetHost(peerAPIURL)}. Open the official Tailscale app and wait until VPN is up.",
            e,
        )
    } catch (e: java.net.SocketTimeoutException) {
        throw SendFailedException(
            "Timed out reaching ${targetHost(peerAPIURL)}. Is official Tailscale connected?",
            e,
        )
    } catch (e: java.net.UnknownHostException) {
        throw SendFailedException(
            "Cannot resolve ${targetHost(peerAPIURL)}. Open the official Tailscale app.",
            e,
        )
    } catch (e: IOException) {
        throw SendFailedException(e.message ?: "send failed", e)
    } finally {
        conn.disconnect()
    }
}

internal fun httpError(code: Int, body: String): String {
    val snippet = body.trim().replace(Regex("\\s+"), " ").take(200)
    return if (snippet.isEmpty()) "PeerAPI HTTP $code" else "PeerAPI HTTP $code: $snippet"
}

internal fun targetHost(peerAPIURL: String): String {
    return try {
        URL(peerAPIURL).host.ifBlank { peerAPIURL }
    } catch (_: Exception) {
        peerAPIURL
    }
}
