package com.sheneyan.tailsend

import androidx.camera.core.ImageProxy
import com.google.zxing.BarcodeFormat
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.NotFoundException
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader

private val hints: Map<DecodeHintType, Any> = mapOf(
    DecodeHintType.POSSIBLE_FORMATS to listOf(BarcodeFormat.QR_CODE),
    DecodeHintType.TRY_HARDER to true,
    DecodeHintType.CHARACTER_SET to "UTF-8",
)

fun decodeQr(image: ImageProxy): String? {
    val plane = image.planes[0]
    val buf = plane.buffer.duplicate()
    val data = ByteArray(buf.remaining())
    buf.get(data)
    val source = PlanarYUVLuminanceSource(
        data,
        plane.rowStride,
        image.height,
        0,
        0,
        image.width,
        image.height,
        false,
    )
    val reader = QRCodeReader()
    return try {
        reader.decode(BinaryBitmap(HybridBinarizer(source)), hints).text
    } catch (_: NotFoundException) {
        try {
            reader.reset()
            reader.decode(BinaryBitmap(HybridBinarizer(source.invert())), hints).text
        } catch (_: Exception) {
            null
        }
    } catch (_: Exception) {
        null
    }
}
