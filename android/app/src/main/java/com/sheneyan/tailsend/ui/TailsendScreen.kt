package com.sheneyan.tailsend.ui

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.sheneyan.tailsend.SendUiState
import com.sheneyan.tailsend.Target
import com.sheneyan.tailsend.osDrawable

private val Bg = Color(0xFF111217)
private val Card = Color(0xFF1C1E26)
private val Accent = Color(0xFF7EB6FF)

@Composable
fun TailsendTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = darkColorScheme(
            background = Bg,
            surface = Card,
            primary = Accent,
        ),
        content = content,
    )
}

@Composable
fun TailsendScreen(
    state: SendUiState,
    onPickFiles: () -> Unit,
    onSendTo: (Target) -> Unit,
    onOpenPair: () -> Unit,
    onClosePair: () -> Unit,
    onPasteChange: (String) -> Unit,
    onHostnameChange: (String) -> Unit,
    onRefresh: () -> Unit,
    onImport: () -> Unit,
    onScan: () -> Unit,
    onScanResult: (String) -> Unit,
    onCloseScan: () -> Unit,
) {
    Surface(color = Bg, modifier = Modifier.fillMaxSize()) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(16.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("Tailsend", fontWeight = FontWeight.SemiBold, fontSize = 22.sp, color = Color.White)
                Spacer(Modifier.weight(1f))
                OutlinedButton(onClick = onOpenPair, enabled = !state.sending) {
                    Text("Pair")
                }
            }
            Text(
                "Parasite sender. Official Tailscale must be connected. No login here.",
                color = Color(0xFF9AA3B2),
                fontSize = 13.sp,
                modifier = Modifier.padding(top = 6.dp, bottom = 16.dp),
            )

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(Card)
                    .clickable(enabled = !state.sending, onClick = onPickFiles)
                    .padding(16.dp),
            ) {
                Column {
                    Text("Click to choose files.", color = Color.White, fontWeight = FontWeight.Medium)
                    Text(
                        if (state.files.isEmpty()) {
                            "Then tap a device. Receive stays in official Tailscale."
                        } else {
                            state.files.joinToString { it.name }
                        },
                        color = Color(0xFF9AA3B2),
                        fontSize = 13.sp,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                }
            }

            if (state.sending) {
                val total = state.progressTotal
                val frac = if (total > 0) (state.progressSent.toFloat() / total.toFloat()).coerceIn(0f, 1f) else 0f
                LinearProgressIndicator(
                    progress = { frac },
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp),
                )
                Text(
                    "${state.progressName} → ${state.sendingTo}  ${state.progressSent}/${if (total >= 0) total else "?"}",
                    color = Color(0xFF9AA3B2),
                    fontSize = 12.sp,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }

            if (state.note.isNotEmpty()) {
                Text(state.note, color = Color(0xFF9AA3B2), fontSize = 13.sp, modifier = Modifier.padding(top = 12.dp))
            }
            if (state.error.isNotEmpty()) {
                Text(state.error, color = Color(0xFFFF8A80), fontSize = 13.sp, modifier = Modifier.padding(top = 8.dp))
            }

            Text(
                "Devices",
                color = Color.White,
                fontWeight = FontWeight.Medium,
                modifier = Modifier.padding(top = 20.dp, bottom = 8.dp),
            )

            if (state.targets.isEmpty()) {
                Text(
                    "No pairing yet. On a computer open Tailsend → Pair phone, then tap Pair here and scan that QR. Or type that computer’s hostname if Tailsend is open there.",
                    color = Color(0xFF9AA3B2),
                    fontSize = 13.sp,
                )
            } else {
                LazyVerticalGrid(
                    columns = GridCells.Adaptive(140.dp),
                    contentPadding = PaddingValues(bottom = 24.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                    modifier = Modifier.fillMaxSize(),
                ) {
                    items(state.targets, key = { it.stableID.ifBlank { it.peerAPIURL } }) { t ->
                        DeviceCard(t, enabled = !state.sending, onClick = { onSendTo(t) })
                    }
                }
            }
        }
    }

    if (state.pairOpen) {
        var pasteOpen by remember { mutableStateOf(false) }
        Dialog(onDismissRequest = onClosePair) {
            Surface(shape = RoundedCornerShape(16.dp), color = Card) {
                Column(Modifier.padding(16.dp)) {
                    Text("Pair from desktop", color = Color.White, fontWeight = FontWeight.SemiBold)
                    Text(
                        "On a computer: Tailsend → Pair phone. Scan that QR. Or type that machine’s short name if its Tailsend window is open.",
                        color = Color(0xFF9AA3B2),
                        fontSize = 13.sp,
                        modifier = Modifier.padding(top = 6.dp, bottom = 12.dp),
                    )
                    Button(onClick = onScan, modifier = Modifier.fillMaxWidth()) {
                        Text("Scan QR")
                    }
                    OutlinedTextField(
                        value = state.hostname,
                        onValueChange = onHostnameChange,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 8.dp),
                        singleLine = true,
                        label = { Text("Computer hostname") },
                    )
                    Button(
                        onClick = onRefresh,
                        enabled = !state.sending,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 8.dp),
                    ) {
                        Text("Refresh from hostname")
                    }
                    TextButton(onClick = onClosePair, modifier = Modifier.fillMaxWidth()) {
                        Text("Close")
                    }
                    TextButton(
                        onClick = { pasteOpen = !pasteOpen },
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(if (pasteOpen) "Hide debug paste" else "Debug: paste JSON")
                    }
                    if (pasteOpen) {
                        OutlinedTextField(
                            value = state.paste,
                            onValueChange = onPasteChange,
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(160.dp),
                            textStyle = androidx.compose.ui.text.TextStyle(
                                fontFamily = FontFamily.Monospace,
                                fontSize = 12.sp,
                                color = Color.White,
                            ),
                        )
                        Button(
                            onClick = onImport,
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 8.dp),
                        ) {
                            Text("Import")
                        }
                    }
                }
            }
        }
    }

    if (state.scanOpen) {
        Dialog(
            onDismissRequest = onCloseScan,
            properties = DialogProperties(usePlatformDefaultWidth = false, dismissOnClickOutside = false),
        ) {
            ScanQr(onResult = onScanResult, onCancel = onCloseScan)
        }
    }
}

@Composable
private fun DeviceCard(t: Target, enabled: Boolean, onClick: () -> Unit) {
    Column(
        modifier = Modifier
            .clip(RoundedCornerShape(12.dp))
            .background(Card)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(14.dp),
    ) {
        Image(
            painter = painterResource(osDrawable(t.os)),
            contentDescription = t.os,
            contentScale = ContentScale.Fit,
            modifier = Modifier
                .size(40.dp)
                .clip(RoundedCornerShape(12.dp)),
        )
        Text(t.label, color = Color.White, fontWeight = FontWeight.Medium, modifier = Modifier.padding(top = 6.dp))
        Text(
            t.os.ifBlank { "device" },
            color = Color(0xFF9AA3B2),
            fontSize = 12.sp,
        )
    }
}


