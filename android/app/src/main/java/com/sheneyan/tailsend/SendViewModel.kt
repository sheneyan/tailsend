package com.sheneyan.tailsend

import android.app.Application
import android.net.Uri
import android.provider.OpenableColumns
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

data class PickedFile(
    val uri: Uri,
    val name: String,
    val size: Long,
)

data class SendUiState(
    val targets: List<Target> = emptyList(),
    val files: List<PickedFile> = emptyList(),
    val paste: String = "",
    val hostname: String = "",
    val pairOpen: Boolean = false,
    val scanOpen: Boolean = false,
    val sending: Boolean = false,
    val sendingTo: String = "",
    val progressName: String = "",
    val progressSent: Long = 0,
    val progressTotal: Long = 0,
    val note: String = "",
    val error: String = "",
)

class SendViewModel(app: Application) : AndroidViewModel(app) {
    private val store = PairStore(app)
    private val _state = MutableStateFlow(SendUiState())
    val state: StateFlow<SendUiState> = _state

    init {
        val targets = store.load()
        _state.update {
            it.copy(
                targets = targets,
                paste = store.loadRaw(),
                note = if (targets.isEmpty()) {
                    "Pair from the desktop app or `tailsend pair`. Official Tailscale must be connected."
                } else {
                    "${targets.size} device(s) from last pair."
                },
            )
        }
    }

    fun setPaste(value: String) {
        _state.update { it.copy(paste = value) }
    }

    fun setHostname(value: String) {
        _state.update { it.copy(hostname = value) }
    }

    fun refreshFromHostname() {
        val name = _state.value.hostname
        viewModelScope.launch {
            _state.update { it.copy(error = "", note = "Asking $name for the device list…") }
            try {
                withContext(Dispatchers.IO) { requestListRefresh(name) }
                _state.update {
                    it.copy(
                        pairOpen = false,
                        note = "Request sent. When Tailscale notifies $PAIR_JSON_NAME, open it in Tailsend. If nothing arrives, open Tailsend on $name, or scan QR.",
                    )
                }
            } catch (e: Exception) {
                _state.update {
                    it.copy(
                        pairOpen = true,
                        error = e.message ?: "refresh failed",
                    )
                }
            }
        }
    }

    fun openPair() {
        _state.update { it.copy(pairOpen = true, scanOpen = false, error = "") }
    }

    fun closePair() {
        _state.update { it.copy(pairOpen = false, scanOpen = false) }
    }

    fun openScan() {
        _state.update { it.copy(scanOpen = true, error = "") }
    }

    fun closeScan() {
        _state.update { it.copy(scanOpen = false) }
    }

    fun cameraDenied() {
        _state.update {
            it.copy(
                scanOpen = false,
                pairOpen = true,
                error = "Camera permission is required to pair. Enable it, or use debug paste.",
            )
        }
    }

    fun importPaste() {
        importRaw(_state.value.paste)
    }

    fun importRaw(raw: String) {
        _state.update { it.copy(paste = raw, scanOpen = false) }
        try {
            val targets = store.save(raw)
            _state.update {
                it.copy(
                    targets = targets,
                    pairOpen = false,
                    error = "",
                    note = "Paired ${targets.size} device(s). Pick files, then tap a device.",
                )
            }
        } catch (e: Exception) {
            _state.update {
                it.copy(
                    pairOpen = true,
                    error = e.message ?: "invalid pairing JSON",
                )
            }
        }
    }

    fun setFiles(uris: List<Uri>) {
        val cr = getApplication<Application>().contentResolver
        val files = uris.map { uri ->
            var name = "file"
            var size = -1L
            cr.query(uri, null, null, null, null)?.use { c ->
                val nameIdx = c.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                val sizeIdx = c.getColumnIndex(OpenableColumns.SIZE)
                if (c.moveToFirst()) {
                    if (nameIdx >= 0) name = c.getString(nameIdx) ?: name
                    if (sizeIdx >= 0 && !c.isNull(sizeIdx)) size = c.getLong(sizeIdx)
                }
            }
            PickedFile(uri, name, size)
        }
        _state.update {
            it.copy(
                files = files,
                note = if (files.isEmpty()) {
                    "Pick files, then tap a device."
                } else {
                    "${files.size} file(s) ready. Tap a device to send."
                },
                error = "",
            )
        }
    }

    fun sendTo(target: Target) {
        val files = _state.value.files
        if (files.isEmpty()) {
            _state.update { it.copy(error = "Pick files first.") }
            return
        }
        if (_state.value.sending) return
        viewModelScope.launch {
            _state.update {
                it.copy(
                    sending = true,
                    sendingTo = target.label,
                    error = "",
                    note = "Sending to ${target.label}…",
                )
            }
            try {
                withContext(Dispatchers.IO) {
                    val cr = getApplication<Application>().contentResolver
                    for (file in files) {
                        cr.openInputStream(file.uri)?.use { input ->
                            putFile(target.peerAPIURL, file.name, file.size, input) { sent, total ->
                                _state.update { s ->
                                    s.copy(
                                        progressName = file.name,
                                        progressSent = sent,
                                        progressTotal = total,
                                    )
                                }
                            }
                        } ?: throw SendFailedException("cannot open ${file.name}")
                    }
                }
                _state.update {
                    it.copy(
                        sending = false,
                        sendingTo = "",
                        progressName = "",
                        note = "Sent ${files.size} file(s) to ${target.label}. Receive is in the official Tailscale app.",
                    )
                }
            } catch (e: Exception) {
                _state.update {
                    it.copy(
                        sending = false,
                        sendingTo = "",
                        error = e.message ?: "send failed",
                        note = "Send failed. Official Tailscale VPN must be up; Tailsend does not embed a node.",
                    )
                }
            }
        }
    }
}
