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
import org.json.JSONArray;
import org.json.JSONException;

/** Offline mail only: no message JavaScript, remote resources, or in-view navigation. */
public final class MailView extends WebView {
    private long owner;
    private int documentRevision;
    private boolean hasDocument;
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
        final int revision = ++documentRevision;
        if (!hasDocument) {
            hasDocument = true;
            loadDataWithBaseURL("https://litterbox.invalid/", document, "text/html", "UTF-8", null);
            return;
        }
        // The document's CSP forbids all page scripts. Enable evaluation only
        // long enough to read native disclosure state, then disable it before
        // loading the next untrusted document.
        getSettings().setJavaScriptEnabled(true);
        evaluateJavascript("Array.from(document.querySelectorAll(\"main.lb-reader > details.lb-previous[data-lb-key]\")).map(d => [d.getAttribute('data-lb-key'), d.open])", value -> {
            if (revision != documentRevision) return;
            getSettings().setJavaScriptEnabled(false);
            String result = document;
            try {
                JSONArray states = new JSONArray(value);
                for (int i = 0; i < states.length(); ++i) {
                    JSONArray state = states.getJSONArray(i);
                    String marker = "<details class='lb-previous' data-lb-key='" + state.getString(0) + "'";
                    result = result.replace(marker + " open", marker);
                    if (state.getBoolean(1)) result = result.replace(marker, marker + " open");
                }
            } catch (JSONException ignored) { }
            loadDataWithBaseURL("https://litterbox.invalid/", result, "text/html", "UTF-8", null);
        });
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
