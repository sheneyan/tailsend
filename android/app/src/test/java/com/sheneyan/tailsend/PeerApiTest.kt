package com.sheneyan.tailsend

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.ByteArrayInputStream
import java.net.ServerSocket
import kotlin.concurrent.thread

class PeerApiTest {
    @Test
    fun putFilePostsToV0Put() {
        val server = ServerSocket(0)
        val port = server.localPort
        var gotLine = ""
        var gotLen = ""
        var gotBody = ByteArray(0)
        val worker = thread {
            server.use { ss ->
                ss.accept().use { sock ->
                    val input = sock.getInputStream()
                    val headers = StringBuilder()
                    val buf = ByteArray(1)
                    while (true) {
                        if (input.read(buf) < 0) break
                        headers.append(buf[0].toInt().toChar())
                        if (headers.endsWith("\r\n\r\n")) break
                    }
                    val text = headers.toString()
                    gotLine = text.lineSequence().first()
                    gotLen = text.lineSequence()
                        .firstOrNull { it.startsWith("Content-Length:", ignoreCase = true) }
                        ?.substringAfter(":")
                        ?.trim()
                        .orEmpty()
                    val n = gotLen.toIntOrNull() ?: 0
                    gotBody = input.readNBytes(n)
                    val resp = "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"
                    sock.getOutputStream().write(resp.toByteArray())
                }
            }
        }
        try {
            val payload = "hello-taildrop".toByteArray()
            putFile(
                "http://127.0.0.1:$port",
                "note.txt",
                payload.size.toLong(),
                ByteArrayInputStream(payload),
            )
            worker.join(5_000)
            assertTrue(gotLine.contains("PUT /v0/put/note.txt"))
            assertEquals(payload.size.toString(), gotLen)
            assertEquals("hello-taildrop", String(gotBody))
        } finally {
            server.close()
            worker.join(1_000)
        }
    }

    @Test
    fun peerAPIPortsMatchGoIPv4() {
        assertEquals(
            listOf(58436, 36467, 45098, 55837, 52376),
            peerAPIPorts("100.64.0.1"),
        )
    }

    @Test
    fun listPleaseJSONEncodesFrom() {
        val raw = listPleaseJSON(listOf("100.64.0.23"))
        val o = org.json.JSONObject(raw)
        assertEquals(1, o.getInt("v"))
        assertEquals("100.64.0.23", o.getJSONArray("from").getString(0))
    }

    @Test
    fun requestListRefreshPutsCommandOnHashedPort() {
        var url = ""
        var name = ""
        var body = ""
        requestListRefresh(
            hostname = "abc",
            resolve = { arrayOf(java.net.InetAddress.getByName("100.64.0.1")) },
            localIPs = { listOf("100.64.0.23") },
            put = { peer, filename, size, input ->
                url = peer
                name = filename
                body = input.bufferedReader().readText()
                size
            },
        )
        assertEquals("http://100.64.0.1:58436", url)
        assertEquals(LIST_PLEASE_NAME, name)
        assertTrue(body.contains("100.64.0.23"))
    }

    @Test
    fun requestListRefreshDnsFailMentionsQr() {
        val err = try {
            requestListRefresh(
                hostname = "abc",
                resolve = { throw java.net.UnknownHostException("abc") },
                localIPs = { listOf("100.64.0.23") },
                put = { _, _, size, _ -> size },
            )
            null
        } catch (e: SendFailedException) {
            e.message ?: ""
        }
        assertTrue(err != null && err.contains("Tailscale DNS") && err.contains("scan QR"))
    }

    @Test
    fun putFileMapsConnectFailure() {
        val err = try {
            putFile(
                "http://127.0.0.1:1",
                "a.txt",
                1,
                ByteArrayInputStream(byteArrayOf(1)),
            )
            null
        } catch (e: SendFailedException) {
            e.message ?: ""
        }
        assertTrue(err != null && err.contains("Tailscale"))
    }
}
