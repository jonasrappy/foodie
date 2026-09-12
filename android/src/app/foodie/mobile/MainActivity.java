package app.foodie.mobile;

import android.app.Activity;
import android.Manifest;
import android.content.pm.PackageManager;
import android.os.Handler;
import android.os.PowerManager;
import android.os.Looper;
import org.json.JSONArray;
import org.json.JSONObject;
import android.app.AlertDialog;
import android.content.Intent;
import android.content.pm.ActivityInfo;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.content.SharedPreferences;
import android.graphics.Color;
import android.graphics.Bitmap;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.provider.Settings;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import java.lang.ref.WeakReference;
import android.view.View;
import android.view.WindowManager;
import android.webkit.CookieManager;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

public class MainActivity extends Activity {
    private static final String SITE = BuildConfig.SITE_URL;
    static final String VOICE_WAKE = "app.foodie.mobile.VOICE_WAKE";
    private static WeakReference<MainActivity> voiceHost = new WeakReference<>(null);
    private WebView web;
    private SharedPreferences prefs;
    private AppUpdater updater;
    private boolean resumed, wakeFromVoice;
    private final Handler voiceHandler = new Handler(Looper.getMainLooper());
    private final Runnable pushVoice = this::dispatchVoiceStatus;
    private final Runnable voiceStatus = new Runnable() {
        public void run() {
            dispatchVoiceStatus();
            voiceHandler.postDelayed(this, 1000);
        }
    };
    static void voiceChanged() {
        MainActivity activity=voiceHost.get();
        if(activity!=null){activity.voiceHandler.removeCallbacks(activity.pushVoice);activity.voiceHandler.post(activity.pushVoice);}
    }
    private void dispatchVoiceStatus() {
        if(!resumed)return;
        VoicePresentation.Frame frame=FoodieService.presentation.snapshot();
        if(!frame.visible&&wakeFromVoice){wakeFromVoice=false;setScreenWake(false);}
        applyWake();
        if(web==null||!trustedPage())return;
        JSONObject detail=new JSONObject();
        try {
            detail.put("running",FoodieService.running);detail.put("status",FoodieService.status);
            detail.put("visible",frame.visible);detail.put("session",frame.session);detail.put("phase",frame.phase);
            detail.put("text",frame.text);detail.put("level",frame.level);
        } catch(Exception ignored){}
        web.evaluateJavascript("window.dispatchEvent(new CustomEvent('mad-voice-status',{detail:"+detail+"}))",null);
    }
    private void setScreenWake(boolean enabled) {
        if(Build.VERSION.SDK_INT>=27)setTurnScreenOn(enabled);
        else if(enabled)getWindow().addFlags(WindowManager.LayoutParams.FLAG_TURN_SCREEN_ON);
        else getWindow().clearFlags(WindowManager.LayoutParams.FLAG_TURN_SCREEN_ON);
    }
    private void receiveVoiceWake(Intent intent) {
        if(intent==null||!VOICE_WAKE.equals(intent.getAction())||!FoodieService.presentation.snapshot().visible)return;
        wakeFromVoice=true;applyLockScreen();setScreenWake(true);applyWake();
        if(web!=null){((InputMethodManager)getSystemService(INPUT_METHOD_SERVICE)).hideSoftInputFromWindow(web.getWindowToken(),0);web.clearFocus();}
        voiceHandler.post(pushVoice);
    }
    @Override protected void onNewIntent(Intent intent) {super.onNewIntent(intent);setIntent(intent);receiveVoiceWake(intent);}
    
    private TextView connectionError;
    private boolean loadFailed;
    private int dp(int value) { return Math.round(value * getResources().getDisplayMetrics().density); }

    @Override public void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        I18n.init(this);
        setRequestedOrientation(ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE);
        prefs = getSharedPreferences("kitchen", MODE_PRIVATE);
        updater = new AppUpdater(this, prefs);
        voiceHost=new WeakReference<>(this);
        applyLockScreen();
        receiveVoiceWake(getIntent());
        LinearLayout layout = new LinearLayout(this);
        layout.setOrientation(LinearLayout.VERTICAL);
        layout.setBackgroundColor(Color.rgb(243,245,239));
        layout.setFitsSystemWindows(true);
        connectionError = new TextView(this);
        connectionError.setText(I18n.text("No connection. Tap here to try again."));
        connectionError.setTextColor(Color.rgb(120,60,30));
        connectionError.setPadding(dp(16),dp(12),dp(16),dp(12));
        connectionError.setVisibility(View.GONE);
        connectionError.setOnClickListener(v -> web.loadUrl(SITE));
        layout.addView(connectionError);
        web = new WebView(this) {
            @Override public InputConnection onCreateInputConnection(EditorInfo info) {
                InputConnection connection = super.onCreateInputConnection(info);
                // Keep the two lists visible when typing in landscape. Samsung's
                // keyboard should use its normal panel instead of a full-screen editor.
                info.imeOptions |= EditorInfo.IME_FLAG_NO_EXTRACT_UI | EditorInfo.IME_FLAG_NO_FULLSCREEN;
                return connection;
            }
        };
        web.setBackgroundColor(Color.rgb(243,245,239));
        WebSettings settings = web.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setUseWideViewPort(true);
        settings.setLoadWithOverviewMode(false);
        settings.setTextZoom(100);
        if (Build.VERSION.SDK_INT >= 29) settings.setForceDark(WebSettings.FORCE_DARK_OFF);
        web.setVerticalScrollBarEnabled(false);
        web.setHorizontalScrollBarEnabled(false);
        web.setOverScrollMode(View.OVER_SCROLL_NEVER);
        settings.setAllowFileAccess(false);
        settings.setAllowContentAccess(false);
        settings.setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        settings.setJavaScriptCanOpenWindowsAutomatically(false);
        settings.setSupportMultipleWindows(false);
        settings.setUserAgentString(settings.getUserAgentString() + " MadTablet/7.0 FoodieAndroid/" + BuildConfig.VERSION_CODE);
        CookieManager.getInstance().setAcceptCookie(true);
        CookieManager.getInstance().setAcceptThirdPartyCookies(web,false);
        WebView.setWebContentsDebuggingEnabled(false);
        web.setDownloadListener((url, agent, disposition, mime, length) -> {
            Uri uri = Uri.parse(url);
            if (trustedPage() && trustedOrigin(uri)) updater.start(uri);
        });
        web.setWebViewClient(new WebViewClient() {
            @Override public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) { return !request.isForMainFrame() || navigate(request.getUrl()); }
            @Override public boolean shouldOverrideUrlLoading(WebView view, String url) { return navigate(Uri.parse(url)); }
            @Override public void onPageStarted(WebView view, String url, Bitmap favicon) { loadFailed = false; }
            @Override public void onPageFinished(WebView view, String url) { if(!loadFailed) connectionError.setVisibility(View.GONE);dispatchVoiceStatus(); }
            @Override public void onReceivedError(WebView view, WebResourceRequest request, WebResourceError error) {
                if (request.isForMainFrame()) { loadFailed = true; connectionError.setVisibility(View.VISIBLE); }
            }
        });
        layout.addView(web, new LinearLayout.LayoutParams(-1,0,1));
        setContentView(layout);
        applyWake();
        immersive();
        web.loadUrl(SITE);
    }
    private boolean navigate(Uri uri) {
        if ("mad-app".equals(uri.getScheme())) {
            Uri current = web.getUrl() == null ? null : Uri.parse(web.getUrl());
            if (trustedPage()) {
                if ("settings".equals(uri.getHost())) showSettings();
                if ("voice".equals(uri.getHost())) {
                    if ("/logout".equals(uri.getPath())) { prefs.edit().remove("voiceToken").apply(); stopService(new Intent(this, FoodieService.class)); }
                    else if("/dismiss".equals(uri.getPath())) {if(FoodieService.running)startService(new Intent(this,FoodieService.class).setAction(FoodieService.DISMISS));}
                    else showVoiceSettings();
                }
            }
            return true;
        }
        if (trustedOrigin(uri)) {
            if ("/api/download/android".equals(uri.getPath()) && trustedPage()) { updater.start(uri); return true; }
            return false;
        }
        if ("https".equals(uri.getScheme())) { try { startActivity(new Intent(Intent.ACTION_VIEW,uri)); } catch (Exception ignored) {} }
        return true;
    }
    private static boolean trustedOrigin(Uri uri) {
        Uri configured=Uri.parse(SITE);
        return uri!=null && "https".equals(uri.getScheme()) && configured.getHost().equalsIgnoreCase(uri.getHost()) && (uri.getPort()==-1?443:uri.getPort())==(configured.getPort()==-1?443:configured.getPort()) && uri.getUserInfo()==null;
    }
    private boolean trustedPage() {
        Uri current = web == null || web.getUrl() == null ? null : Uri.parse(web.getUrl());
        return trustedOrigin(current);
    }
    private void showVoiceSettings() {
        String error = prefs.getString("voiceError", "");
        String message = FoodieService.running ? FoodieService.status : error.isEmpty() ? I18n.text("Say Hey Foodie, even when the screen is off. Foodie will ask what to add and reply in English.\n\nThe wake word is detected locally on this device. Android's speech service recognizes your reply and may send the short command to Google or Samsung. Your Foodie server receives only text.\n\nUses media volume and additional battery power.") : error;
        new AlertDialog.Builder(this).setTitle("Hey Foodie").setMessage(message)
            .setPositiveButton(FoodieService.running ? I18n.text("Turn off") : I18n.text("Turn on"), (dialog, which) -> {
                if (FoodieService.running) { startService(new Intent(this, FoodieService.class).setAction(FoodieService.STOP)); }
                else enableVoice();
            })
            .setNeutralButton(I18n.text("Settings"), (dialog, which) -> new AlertDialog.Builder(this).setTitle("Foodie-indstillinger").setItems(new String[]{I18n.text("Voice and text-to-speech"), I18n.text("Speech recognition"), I18n.text("Battery and permissions"), I18n.text("Latest voice error"), I18n.text("Screen wake · Display over other apps")}, (d, index) -> {
                if (index == 3) { new AlertDialog.Builder(this).setTitle(I18n.text("Latest voice error")).setMessage(prefs.getString("voiceDiagnostic", I18n.text("No errors recorded in this version."))).setPositiveButton(I18n.text("Close"), null).show(); return; }
                if(index==4){openVoiceScreenSettings(false);return;}
                try { startActivity(index == 0 ? new Intent("com.android.settings.TTS_SETTINGS") : index == 1 ? new Intent(Settings.ACTION_VOICE_INPUT_SETTINGS) : new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:" + getPackageName()))); }
                catch (Exception ignored) { startActivity(new Intent(Settings.ACTION_SETTINGS)); }
            }).setNegativeButton(I18n.text("Close"), null).show())
            .setNegativeButton(I18n.text("Close"), null).show();
    }
    private void enableVoice() {
        if (!trustedPage()) return;
        web.evaluateJavascript("localStorage.getItem('mad.token')", value -> {
            try {
                String deviceToken = new JSONArray("[" + value + "]").optString(0, "");
                if (!deviceToken.startsWith("device.")) { Toast.makeText(this, I18n.text("Log in with the household password first."), Toast.LENGTH_LONG).show(); return; }
                prefs.edit().putBoolean("voiceUseAndroid", true).putString("voiceToken", deviceToken).remove("voiceError").apply();
                java.util.ArrayList<String> permissions = new java.util.ArrayList<>();
                if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) permissions.add(Manifest.permission.RECORD_AUDIO);
                if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) permissions.add(Manifest.permission.POST_NOTIFICATIONS);
                if (!permissions.isEmpty()) requestPermissions(permissions.toArray(new String[0]), 42);
                else startVoice();
            } catch (Exception ignored) { Toast.makeText(this, I18n.text("Open the lists and try again."), Toast.LENGTH_LONG).show(); }
        });
    }
    private void startVoice() {
        if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) { Toast.makeText(this, I18n.text("Foodie needs microphone access to listen."), Toast.LENGTH_LONG).show(); return; }
        if(Build.VERSION.SDK_INT>=29&&!Settings.canDrawOverlays(this)&&!prefs.getBoolean("voiceScreenAsked",false)){
            prefs.edit().putBoolean("voiceScreenAsked",true).apply();
            new AlertDialog.Builder(this).setTitle(I18n.text("Wake Foodie by voice"))
                .setMessage(I18n.text("Allow Foodie to display over other apps so it can open and wake the screen when you say Hey Foodie."))
                .setPositiveButton(I18n.text("Open settings"),(dialog,which)->openVoiceScreenSettings(true))
                .setNegativeButton(I18n.text("Not now"),(dialog,which)->startVoice()).show();return;
        }
        PowerManager power = (PowerManager) getSystemService(POWER_SERVICE);
        if (!power.isIgnoringBatteryOptimizations(getPackageName()) && !prefs.getBoolean("voiceBatteryAsked", false)) {
            prefs.edit().putBoolean("voiceBatteryAsked", true).apply();
            try { startActivityForResult(new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:" + getPackageName())), 43); return; }
            catch (Exception ignored) {}
        }
        Intent intent = new Intent(this, FoodieService.class).setAction(FoodieService.START);
        try { if (Build.VERSION.SDK_INT >= 26) startForegroundService(intent); else startService(intent); }
        catch (Exception ignored) { Toast.makeText(this, I18n.text("Open the app and turn Foodie on again."), Toast.LENGTH_LONG).show(); }
    }
    private void openVoiceScreenSettings(boolean continueStart) {
        Intent intent=new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,Uri.parse("package:"+getPackageName()));
        try {if(continueStart)startActivityForResult(intent,44);else startActivity(intent);}
        catch(RuntimeException error){Toast.makeText(this,I18n.text("Find Foodie under Android's Display over other apps settings."),Toast.LENGTH_LONG).show();if(continueStart)startVoice();}
    }
    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
        if (request == AppUpdater.INSTALL_PERMISSION) updater.permissionReturned();
        if (request == 43 || request == 44) voiceHandler.postDelayed(this::startVoice, 250);
    }
    @Override public void onRequestPermissionsResult(int request, String[] permissions, int[] results) {
        super.onRequestPermissionsResult(request, permissions, results);
        if (request == 42) startVoice();
    }
    private void applyLockScreen() {
        boolean enabled = prefs.getBoolean("showLocked",true);
        if (Build.VERSION.SDK_INT >= 27) setShowWhenLocked(enabled);
        else if(enabled) getWindow().addFlags(WindowManager.LayoutParams.FLAG_SHOW_WHEN_LOCKED);
        else getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SHOW_WHEN_LOCKED);
    }
    private void applyWake() {
        boolean enabled = prefs.getBoolean("keepAwake",false)||FoodieService.presentation.snapshot().visible;
        if(enabled==((getWindow().getAttributes().flags&WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)!=0))return;
        if(enabled) getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        else getWindow().clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);

    }
    private void showSettings() {
        String lock = prefs.getBoolean("showLocked",true) ? I18n.text("Show over lock screen: on") : I18n.text("Show over lock screen: off");
        String[] items = {prefs.getBoolean("keepAwake",false) ? I18n.text("Keep screen on: on") : I18n.text("Keep screen on: off"),lock,I18n.text("Choose default home app"),I18n.text("Pin this app"),I18n.text("Reload lists"),I18n.text("Open Android settings"),I18n.text("About Foodie"),I18n.text("Hey Foodie voice control")};
        new AlertDialog.Builder(this).setTitle(I18n.text("Kitchen tablet")).setItems(items,(dialog,which) -> {
            if(which==0) { prefs.edit().putBoolean("keepAwake",!prefs.getBoolean("keepAwake",false)).apply(); applyWake(); }
            if(which==1) { prefs.edit().putBoolean("showLocked",!prefs.getBoolean("showLocked",true)).apply(); applyLockScreen(); }
            if(which==2) { try { startActivity(new Intent(Settings.ACTION_HOME_SETTINGS)); } catch(Exception ignored) { startActivity(new Intent(Settings.ACTION_SETTINGS)); } }
            if(which==3) { try { startLockTask(); } catch(Exception ignored) { Toast.makeText(this,I18n.text("Enable app pinning in Android settings."),Toast.LENGTH_LONG).show(); } }
            if(which==4) web.loadUrl(SITE);
            if(which==5) startActivity(new Intent(Settings.ACTION_SETTINGS));
            if(which==7) showVoiceSettings();
            if(which==6) new AlertDialog.Builder(this).setTitle("Foodie " + BuildConfig.VERSION_NAME).setMessage(I18n.text("Shared shopping lists and meal requests.\n\nChoose Foodie as your home app for a dedicated kitchen tablet. You can change this in Settings > Choose default home app.\n\nWhen lock screen display is enabled, anyone holding the tablet can view and edit the lists while Foodie is visible. Other apps still require normal unlocking.\n\nAn internet connection is needed to sync lists. Login is remembered until the household password changes or app data is cleared.")).setPositiveButton("OK",null).show();
        }).setNegativeButton(I18n.text("Close"),null).show();
    }
    @Override protected void onResume() { super.onResume();resumed=true; if(updater!=null)updater.resume(); voiceHandler.removeCallbacks(voiceStatus); voiceHandler.post(voiceStatus); immersive(); if(web!=null) { web.onResume(); web.evaluateJavascript("window.dispatchEvent(new Event('online'))",null); } }
    @Override protected void onPause() {resumed=false; if(updater!=null)updater.pause(); voiceHandler.removeCallbacks(voiceStatus);voiceHandler.removeCallbacks(pushVoice); if(web!=null) { CookieManager.getInstance().flush(); web.onPause(); } super.onPause(); }
    @Override public void onBackPressed() {
        if(web!=null) web.evaluateJavascript("window.dispatchEvent(new Event('mad-back'))",null);
    }
    @Override public void onWindowFocusChanged(boolean focused) { super.onWindowFocusChanged(focused); if(focused) immersive(); }
    private void immersive() {
        if(Build.VERSION.SDK_INT>=30) {
            WindowInsetsController controller=getWindow().getInsetsController();
            if(controller!=null){controller.setSystemBarsBehavior(WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);controller.hide(WindowInsets.Type.systemBars());}
        } else {
            getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY | View.SYSTEM_UI_FLAG_FULLSCREEN | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION);
        }
    }
    @Override protected void onDestroy() {if(updater!=null)updater.pause();if(voiceHost.get()==this)voiceHost.clear(); voiceHandler.removeCallbacksAndMessages(null); if(web!=null) web.destroy(); super.onDestroy(); }
}
