package com.sheneyan.tailsend

enum class OsKind {
    Mac,
    Win,
    Linux,
    Android,
    Ios,
    Other,
}

fun osKind(os: String): OsKind {
    val s = os.lowercase()
    return when {
        s.contains("mac") || s == "darwin" -> OsKind.Mac
        s.contains("win") -> OsKind.Win
        s.contains("android") -> OsKind.Android
        s.contains("ios") || s.contains("iphone") || s.contains("ipad") -> OsKind.Ios
        s.contains("linux") -> OsKind.Linux
        else -> OsKind.Other
    }
}

fun osDrawable(os: String): Int {
    return when (osKind(os)) {
        OsKind.Mac -> R.drawable.os_mac
        OsKind.Win -> R.drawable.os_win
        OsKind.Linux -> R.drawable.os_linux
        OsKind.Android -> R.drawable.os_android
        OsKind.Ios -> R.drawable.os_ios
        OsKind.Other -> R.drawable.os_other
    }
}
