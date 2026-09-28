package io.github.komzpa.app

import android.content.Intent
import android.net.Uri
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {
    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, "io.github.komzpa.litterbox/gmail")
            .setMethodCallHandler { call, result ->
                if (call.method != "openThread") {
                    result.notImplemented()
                    return@setMethodCallHandler
                }
                val threadId = call.argument<String>("threadId")
                val url = call.argument<String>("url")
                if (threadId.isNullOrBlank() || url.isNullOrBlank()) {
                    result.error("invalid_arguments", "threadId and url are required", null)
                    return@setMethodCallHandler
                }
                val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url)).setPackage("com.google.android.gm")
                try {
                    startActivity(intent)
                    result.success(null)
                } catch (_: Exception) {
                    startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
                    result.success(null)
                }
            }
    }
}
