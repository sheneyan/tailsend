package com.sheneyan.tailsend

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Bundle
import android.provider.OpenableColumns
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.core.content.ContextCompat
import com.sheneyan.tailsend.ui.TailsendScreen
import com.sheneyan.tailsend.ui.TailsendTheme

class MainActivity : ComponentActivity() {
    private val vm: SendViewModel by viewModels()

    private val pickFiles = registerForActivityResult(
        ActivityResultContracts.GetMultipleContents(),
    ) { uris ->
        if (uris.isNotEmpty()) vm.setFiles(uris)
    }

    private val requestCamera = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) vm.openScan() else vm.cameraDenied()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            val state by vm.state.collectAsState()
            TailsendTheme {
                TailsendScreen(
                    state = state,
                    onPickFiles = { pickFiles.launch("*/*") },
                    onSendTo = vm::sendTo,
                    onOpenPair = vm::openPair,
                    onClosePair = vm::closePair,
                    onPasteChange = vm::setPaste,
                    onHostnameChange = vm::setHostname,
                    onRefresh = vm::refreshFromHostname,
                    onImport = vm::importPaste,
                    onScan = ::startScan,
                    onScanResult = vm::importRaw,
                    onCloseScan = vm::closeScan,
                )
            }
        }
        handleImportIntent(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleImportIntent(intent)
    }

    private fun handleImportIntent(intent: Intent?) {
        if (intent == null) return
        val uri: Uri? = when (intent.action) {
            Intent.ACTION_VIEW -> intent.data
            Intent.ACTION_SEND -> {
                @Suppress("DEPRECATION")
                intent.getParcelableExtra(Intent.EXTRA_STREAM)
            }
            else -> null
        }
        if (uri == null) {
            val text = intent.getStringExtra(Intent.EXTRA_TEXT)
            if (!text.isNullOrBlank() && (intent.action == Intent.ACTION_SEND || intent.action == Intent.ACTION_VIEW)) {
                vm.importRaw(text)
            }
            return
        }
        val name = displayName(uri)
        val raw = try {
            contentResolver.openInputStream(uri)?.bufferedReader()?.readText()
        } catch (_: Exception) {
            null
        }
        if (raw.isNullOrBlank()) return
        if (name.isNotEmpty() && !isPairJsonFilename(name)) {
            try {
                parsePairingJson(raw)
            } catch (_: Exception) {
                return
            }
        }
        vm.importRaw(raw)
    }

    private fun displayName(uri: Uri): String {
        contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c ->
            val idx = c.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            if (idx >= 0 && c.moveToFirst()) {
                return c.getString(idx).orEmpty()
            }
        }
        return uri.lastPathSegment.orEmpty()
    }

    private fun startScan() {
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) ==
            PackageManager.PERMISSION_GRANTED
        ) {
            vm.openScan()
        } else {
            requestCamera.launch(Manifest.permission.CAMERA)
        }
    }
}
