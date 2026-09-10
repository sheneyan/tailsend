package com.sheneyan.tailsend

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class PairingTest {
    private val sample = """
        [
          {
            "stableID": "n-self",
            "hostname": "macbook",
            "dnsName": "macbook.tailnet.ts.net",
            "ips": ["100.64.0.1"],
            "os": "macOS",
            "online": true,
            "peerAPIURL": "http://100.64.0.1:41641"
          },
          {
            "stableID": "n-phone",
            "hostname": "pixel",
            "dnsName": "pixel.tailnet.ts.net",
            "ips": ["100.64.0.2"],
            "os": "android",
            "online": true,
            "peerAPIURL": "http://100.64.0.2:41641"
          }
        ]
    """.trimIndent()

    @Test
    fun parsesCompactQRJSON() {
        val json = """[{"stableID":"n-self","hostname":"macbook","os":"macOS","peerAPIURL":"http://100.64.0.1:41641"}]"""
        val got = parsePairingJson(json)
        assertEquals(1, got.size)
        assertEquals("macbook", got[0].hostname)
        assertEquals("http://100.64.0.1:41641", got[0].peerAPIURL)
    }

    @Test
    fun parsesExportTargetsJSON() {
        val got = parsePairingJson(sample)
        assertEquals(2, got.size)
        assertEquals("macbook", got[0].hostname)
        assertEquals("http://100.64.0.1:41641", got[0].peerAPIURL)
        assertEquals("pixel", got[1].hostname)
    }

    @Test
    fun dropsMissingPeerAPI() {
        val json = """[{"hostname":"ghost","stableID":"n-x"}]"""
        assertThrows(IllegalArgumentException::class.java) { parsePairingJson(json) }
    }

    @Test
    fun rejectsNonArray() {
        assertThrows(IllegalArgumentException::class.java) { parsePairingJson("{}") }
        assertThrows(IllegalArgumentException::class.java) { parsePairingJson("") }
    }

    @Test
    fun pathEncodeKeepsSafeAndPercentEncodesSpace() {
        assertEquals("photo.jpg", pathEncode("photo.jpg"))
        assertEquals("my%20file.txt", pathEncode("my file.txt"))
        assertTrue(pathEncode("照片.png").startsWith("%"))
    }

    @Test
    fun parsesPairJsonFilenamePayload() {
        val got = parsePairingJson(sample)
        assertEquals("macbook", got[0].hostname)
        assertTrue(isPairJsonFilename("tailsend-pair.json"))
        assertTrue(isPairJsonFilename("/storage/emulated/0/Download/tailsend-pair.json"))
        assertTrue(!isPairJsonFilename("hello.txt"))
    }

    @Test
    fun looksLikePairShareOnlyForJsonLike() {
        assertTrue(looksLikePairShare("tailsend-pair.json", "application/json"))
        assertTrue(looksLikePairShare("list.json", "application/json"))
        assertTrue(looksLikePairShare("note.txt", "text/plain"))
        assertTrue(!looksLikePairShare("photo.jpg", "image/jpeg"))
        assertTrue(!looksLikePairShare("clip.mp4", "video/mp4"))
        assertTrue(!looksLikePairShare("doc.pdf", "application/pdf"))
    }

    @Test
    fun putUrlJoinsPeerAPI() {
        assertEquals(
            "http://100.64.0.1:41641/v0/put/photo.jpg",
            putUrl("http://100.64.0.1:41641/", "photo.jpg"),
        )
    }
}
