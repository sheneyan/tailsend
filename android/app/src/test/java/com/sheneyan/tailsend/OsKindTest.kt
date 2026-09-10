package com.sheneyan.tailsend

import org.junit.Assert.assertEquals
import org.junit.Test

class OsKindTest {
    @Test
    fun mapsTailscaleOsStrings() {
        assertEquals(OsKind.Mac, osKind("macOS"))
        assertEquals(OsKind.Mac, osKind("darwin"))
        assertEquals(OsKind.Win, osKind("windows"))
        assertEquals(OsKind.Linux, osKind("linux"))
        assertEquals(OsKind.Android, osKind("android"))
        assertEquals(OsKind.Ios, osKind("iOS"))
        assertEquals(OsKind.Other, osKind("freebsd"))
    }
}
