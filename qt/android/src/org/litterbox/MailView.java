package org.litterbox;

import android.app.Activity;
import android.content.Context;
import android.graphics.Color;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.CookieManager;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import java.io.ByteArrayInputStream;

/** Offline mail only: no message JavaScript, remote resources, or in-view navigation. */
public final class MailView extends WebView {
    private long owner;
    private static native void linkClicked(long owner, String url);

    public MailView(Context context, long owner) {
        super(context);
        this.owner = owner;
        setBackgroundColor(Color.WHITE);
        setVisibility(View.GONE);
        WebSettings settings = getSettings();
        settings.setJavaScriptEnabled(false);
        settings.setJavaScriptCanOpenWindowsAutomatically(false);
        settings.setAllowFileAccess(false);
        settings.setAllowContentAccess(false);
        settings.setBlockNetworkLoads(true);
        settings.setDomStorageEnabled(false);
        settings.setCacheMode(WebSettings.LOAD_NO_CACHE);
        settings.setUseWideViewPort(true);
        settings.setLoadWithOverviewMode(false);
        settings.setSupportZoom(false);
        settings.setMinimumFontSize(14);
        CookieManager.getInstance().setAcceptThirdPartyCookies(this, false);
        setWebViewClient(new WebViewClient() {
            private boolean intercept(String url) {
                if (MailView.this.owner != 0) linkClicked(MailView.this.owner, url);
                return true;
            }
            @Override public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                if (request.isForMainFrame() && request.hasGesture()) return intercept(request.getUrl().toString());
                return true;
            }
            @Override public boolean shouldOverrideUrlLoading(WebView view, String url) {
                return intercept(url);
            }
            @Override public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest request) {
                if (request.getUrl().toString().startsWith("data:image/")) return null;
                return new WebResourceResponse("text/plain", "UTF-8", new ByteArrayInputStream(new byte[0]));
            }
        });
        // File/download links are delivered to Qt's existing cached-file opener.
        setDownloadListener((url, userAgent, disposition, mime, length) -> {
            if (this.owner != 0) linkClicked(this.owner, url);
        });
        ((Activity) context).addContentView(this, new FrameLayout.LayoutParams(1, 1));
    }

    public void loadMail(String document) {
        loadDataWithBaseURL("https://litterbox.invalid/", document, "text/html", "UTF-8", null);
    }
    public void place(int x, int y, int width, int height, boolean visible) {
        FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(width, height);
        params.leftMargin = x;
        params.topMargin = y;
        setLayoutParams(params);
        setVisibility(visible ? View.VISIBLE : View.GONE);
    }
    public void dispose() {
        owner = 0;
        stopLoading();
        ((ViewGroup) getParent()).removeView(this);
        destroy();
    }
}
