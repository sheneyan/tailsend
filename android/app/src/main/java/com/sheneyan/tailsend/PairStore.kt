package com.sheneyan.tailsend

import android.content.Context

class PairStore(ctx: Context) {
    private val prefs = ctx.applicationContext.getSharedPreferences("pair", Context.MODE_PRIVATE)

    fun loadRaw(): String = prefs.getString(KEY, "").orEmpty()

    fun load(): List<Target> {
        val raw = loadRaw()
        if (raw.isBlank()) return emptyList()
        return try {
            parsePairingJson(raw)
        } catch (_: Exception) {
            emptyList()
        }
    }

    fun save(raw: String): List<Target> {
        val targets = parsePairingJson(raw)
        prefs.edit().putString(KEY, raw.trim()).apply()
        return targets
    }

    fun clear() {
        prefs.edit().remove(KEY).apply()
    }

    companion object {
        private const val KEY = "json"
    }
}
