package com.sheneyan.tailsend.ui

import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.sheneyan.tailsend.decodeQr
import java.util.concurrent.atomic.AtomicBoolean

@Composable
fun ScanQr(onResult: (String) -> Unit, onCancel: () -> Unit) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val done = remember { AtomicBoolean(false) }
    var failed by remember { mutableStateOf("") }

    Box(Modifier.fillMaxSize().background(Color.Black)) {
        AndroidView(
            modifier = Modifier.fillMaxSize(),
            factory = { ctx ->
                PreviewView(ctx).apply { implementationMode = PreviewView.ImplementationMode.COMPATIBLE }
            },
            update = { previewView ->
                val future = ProcessCameraProvider.getInstance(context)
                future.addListener(
                    {
                        try {
                            val provider = future.get()
                            val preview = Preview.Builder().build().also {
                                it.surfaceProvider = previewView.surfaceProvider
                            }
                            val analysis = ImageAnalysis.Builder()
                                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                                .build()
                            analysis.setAnalyzer(ContextCompat.getMainExecutor(context)) { image ->
                                if (done.get()) {
                                    image.close()
                                    return@setAnalyzer
                                }
                                val text = try {
                                    decodeQr(image)
                                } finally {
                                    image.close()
                                }
                                if (!text.isNullOrBlank() && done.compareAndSet(false, true)) {
                                    onResult(text)
                                }
                            }
                            provider.unbindAll()
                            provider.bindToLifecycle(
                                lifecycleOwner,
                                CameraSelector.DEFAULT_BACK_CAMERA,
                                preview,
                                analysis,
                            )
                        } catch (e: Exception) {
                            failed = e.message ?: "camera failed"
                        }
                    },
                    ContextCompat.getMainExecutor(context),
                )
            },
        )
        Text(
            failed.ifBlank { "Point at the Pair phone QR from the desktop app." },
            color = Color.White,
            modifier = Modifier
                .align(Alignment.TopCenter)
                .padding(24.dp),
        )
        TextButton(
            onClick = onCancel,
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .padding(24.dp),
        ) {
            Text("Cancel", color = Color.White)
        }
    }

    DisposableEffect(Unit) {
        onDispose { done.set(true) }
    }
}
