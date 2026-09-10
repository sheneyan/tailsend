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
        handleIncomingIntent(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleIncomingIntent(intent)
    }

    private fun handleIncomingIntent(intent: Intent?) {
        if (intent == null) return
        when (intent.action) {
            Intent.ACTION_VIEW -> {
                val uri = intent.data ?: return
                if (looksLikePairShare(displayName(uri), intent.type)) {
                    importPairUri(uri)
                } else {
                    takeRead(uri)
                    vm.setFiles(listOf(uri))
                }
            }
            Intent.ACTION_SEND -> {
                val uri = extraStream(intent)
                if (uri != null) {
                    if (looksLikePairShare(displayName(uri), intent.type)) {
                        importPairUri(uri)
                    } else {
                        takeRead(uri)
                        vm.setFiles(listOf(uri))
                    }
                    return
                }
                val text = intent.getStringExtra(Intent.EXTRA_TEXT)
                if (!text.isNullOrBlank()) {
                    try {
                        parsePairingJson(text)
                        vm.importRaw(text)
                    } catch (_: Exception) {
                        // text/clipboard share is out of scope
                    }
                }
            }
            Intent.ACTION_SEND_MULTIPLE -> {
                val uris = extraStreams(intent)
                if (uris.size == 1 && looksLikePairShare(displayName(uris[0]), intent.type)) {
                    importPairUri(uris[0])
                    return
                }
                if (uris.isNotEmpty()) {
                    uris.forEach(::takeRead)
                    vm.setFiles(uris)
                }
            }
        }
    }

    private fun extraStream(intent: Intent): Uri? {
        @Suppress("DEPRECATION")
        return intent.getParcelableExtra(Intent.EXTRA_STREAM)
    }

    private fun extraStreams(intent: Intent): List<Uri> {
        @Suppress("DEPRECATION")
        val list = intent.getParcelableArrayListExtra<Uri>(Intent.EXTRA_STREAM)
        return list?.filterNotNull().orEmpty()
    }

    private fun takeRead(uri: Uri) {
        try {
            contentResolver.takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
        } catch (_: SecurityException) {
            // Share-sheet grants are often one-shot; FLAG_GRANT_READ_URI_PERMISSION on the intent is enough.
        }
    }

    private fun importPairUri(uri: Uri) {
        val raw = try {
            contentResolver.openInputStream(uri)?.bufferedReader()?.readText()
        } catch (_: Exception) {
            null
        }
        if (raw.isNullOrBlank()) return
        try {
            parsePairingJson(raw)
        } catch (_: Exception) {
            takeRead(uri)
            vm.setFiles(listOf(uri))
            return
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
