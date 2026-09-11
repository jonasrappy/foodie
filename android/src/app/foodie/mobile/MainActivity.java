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
        voiceHost=new WeakReference<>(this);
        applyLockScreen();
        receiveVoiceWake(getIntent());
        LinearLayout layout = new LinearLayout(this);
        layout.setOrientation(LinearLayout.VERTICAL);
        layout.setBackgroundColor(Color.rgb(243,245,239));
        layout.setFitsSystemWindows(true);
        connectionError = new TextView(this);
        connectionError.setText(I18n.text("Ingen forbindelse. Tryk her for at prøve igen."));
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
        settings.setUserAgentString(settings.getUserAgentString() + " MadTablet/7.0");
        CookieManager.getInstance().setAcceptCookie(true);
        CookieManager.getInstance().setAcceptThirdPartyCookies(web,false);
        WebView.setWebContentsDebuggingEnabled(false);
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
        if (trustedOrigin(uri)) return false;
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
        String message = FoodieService.running ? FoodieService.status : error.isEmpty() ? I18n.text("Sig Hey Foodie, også med slukket skærm. Jeg spørger, hvad du vil tilføje, og svarer på dansk.\n\nAktiveringsordet genkendes lokalt på tabletten. Dit efterfølgende svar genkendes af Androids danske taletjeneste, som kan sende den korte kommando til Google eller Samsung. Jeres server modtager kun teksten.\n\nBruger medielyd og ekstra batteri.") : error;
        new AlertDialog.Builder(this).setTitle("Hey Foodie").setMessage(message)
            .setPositiveButton(FoodieService.running ? I18n.text("Slå fra") : I18n.text("Slå til"), (dialog, which) -> {
                if (FoodieService.running) { startService(new Intent(this, FoodieService.class).setAction(FoodieService.STOP)); }
                else enableVoice();
            })
            .setNeutralButton(I18n.text("Indstillinger"), (dialog, which) -> new AlertDialog.Builder(this).setTitle("Foodie-indstillinger").setItems(new String[]{I18n.text("Dansk stemme og oplæsning"), I18n.text("Talegenkendelse"), I18n.text("Appens batteri og tilladelser"), I18n.text("Seneste stemmefejl"), I18n.text("Skærmvækning · Vis oven på andre apps")}, (d, index) -> {
                if (index == 3) { new AlertDialog.Builder(this).setTitle(I18n.text("Seneste stemmefejl")).setMessage(prefs.getString("voiceDiagnostic", I18n.text("Ingen fejl registreret i denne version."))).setPositiveButton(I18n.text("Luk"), null).show(); return; }
                if(index==4){openVoiceScreenSettings(false);return;}
                try { startActivity(index == 0 ? new Intent("com.android.settings.TTS_SETTINGS") : index == 1 ? new Intent(Settings.ACTION_VOICE_INPUT_SETTINGS) : new Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, Uri.parse("package:" + getPackageName()))); }
                catch (Exception ignored) { startActivity(new Intent(Settings.ACTION_SETTINGS)); }
            }).setNegativeButton(I18n.text("Luk"), null).show())
            .setNegativeButton(I18n.text("Luk"), null).show();
    }
    private void enableVoice() {
        if (!trustedPage()) return;
        web.evaluateJavascript("localStorage.getItem('mad.token')", value -> {
            try {
                String deviceToken = new JSONArray("[" + value + "]").optString(0, "");
                if (!deviceToken.startsWith("device.")) { Toast.makeText(this, I18n.text("Log ind med husets kode først."), Toast.LENGTH_LONG).show(); return; }
                prefs.edit().putBoolean("voiceUseAndroid", true).putString("voiceToken", deviceToken).remove("voiceError").apply();
                java.util.ArrayList<String> permissions = new java.util.ArrayList<>();
                if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) permissions.add(Manifest.permission.RECORD_AUDIO);
                if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) permissions.add(Manifest.permission.POST_NOTIFICATIONS);
                if (!permissions.isEmpty()) requestPermissions(permissions.toArray(new String[0]), 42);
                else startVoice();
            } catch (Exception ignored) { Toast.makeText(this, I18n.text("Åbn listerne og prøv igen."), Toast.LENGTH_LONG).show(); }
        });
    }
    private void startVoice() {
        if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) { Toast.makeText(this, I18n.text("Foodie skal have adgang til mikrofonen for at lytte."), Toast.LENGTH_LONG).show(); return; }
        if(Build.VERSION.SDK_INT>=29&&!Settings.canDrawOverlays(this)&&!prefs.getBoolean("voiceScreenAsked",false)){
            prefs.edit().putBoolean("voiceScreenAsked",true).apply();
            new AlertDialog.Builder(this).setTitle(I18n.text("Væk Foodie med stemmen"))
                .setMessage(I18n.text("Tillad Foodie at vise oven på andre apps, så figuren kan komme frem og vække skærmen, når du siger Hey Foodie."))
                .setPositiveButton(I18n.text("Åbn indstillinger"),(dialog,which)->openVoiceScreenSettings(true))
                .setNegativeButton(I18n.text("Ikke nu"),(dialog,which)->startVoice()).show();return;
        }
        PowerManager power = (PowerManager) getSystemService(POWER_SERVICE);
        if (!power.isIgnoringBatteryOptimizations(getPackageName()) && !prefs.getBoolean("voiceBatteryAsked", false)) {
            prefs.edit().putBoolean("voiceBatteryAsked", true).apply();
            try { startActivityForResult(new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS, Uri.parse("package:" + getPackageName())), 43); return; }
            catch (Exception ignored) {}
        }
        Intent intent = new Intent(this, FoodieService.class).setAction(FoodieService.START);
        try { if (Build.VERSION.SDK_INT >= 26) startForegroundService(intent); else startService(intent); }
        catch (Exception ignored) { Toast.makeText(this, I18n.text("Åbn appen og slå Foodie til igen."), Toast.LENGTH_LONG).show(); }
    }
    private void openVoiceScreenSettings(boolean continueStart) {
        Intent intent=new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,Uri.parse("package:"+getPackageName()));
        try {if(continueStart)startActivityForResult(intent,44);else startActivity(intent);}
        catch(RuntimeException error){Toast.makeText(this,I18n.text("Find Foodie under Androids Vis oven på andre apps."),Toast.LENGTH_LONG).show();if(continueStart)startVoice();}
    }
    @Override protected void onActivityResult(int request, int result, Intent data) {
        super.onActivityResult(request, result, data);
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
        String lock = prefs.getBoolean("showLocked",true) ? I18n.text("Vis over låseskærm: til") : I18n.text("Vis over låseskærm: fra");
        String[] items = {prefs.getBoolean("keepAwake",false) ? I18n.text("Hold skærmen tændt: til") : I18n.text("Hold skærmen tændt: fra"),lock,I18n.text("Vælg fast startapp / hjemmeskærm"),I18n.text("Fastgør appen på skærmen"),I18n.text("Genindlæs lister"),I18n.text("Åbn Android-indstillinger"),I18n.text("Om Foodie"),I18n.text("Hey Foodie – stemmestyring")};
        new AlertDialog.Builder(this).setTitle(I18n.text("Køkkentablet")).setItems(items,(dialog,which) -> {
            if(which==0) { prefs.edit().putBoolean("keepAwake",!prefs.getBoolean("keepAwake",false)).apply(); applyWake(); }
            if(which==1) { prefs.edit().putBoolean("showLocked",!prefs.getBoolean("showLocked",true)).apply(); applyLockScreen(); }
            if(which==2) { try { startActivity(new Intent(Settings.ACTION_HOME_SETTINGS)); } catch(Exception ignored) { startActivity(new Intent(Settings.ACTION_SETTINGS)); } }
            if(which==3) { try { startLockTask(); } catch(Exception ignored) { Toast.makeText(this,I18n.text("Aktivér appfastgørelse i Android-indstillinger."),Toast.LENGTH_LONG).show(); } }
            if(which==4) web.loadUrl(SITE);
            if(which==5) startActivity(new Intent(Settings.ACTION_SETTINGS));
            if(which==7) showVoiceSettings();
            if(which==6) new AlertDialog.Builder(this).setTitle("Foodie " + BuildConfig.VERSION_NAME).setMessage(I18n.text("Vores indkøbsliste og madønsker.\n\nVælg Foodie som startapp, hvis tabletten skal være en fast køkkenskærm. Du kan altid skifte tilbage via Indstillinger → Vælg fast startapp.\n\nNår låseskærmsvisning er slået til, kan alle med tabletten se og redigere listerne, mens appen er fremme. Andre apps kræver stadig normal oplåsning.\n\nInternet kræves for at hente og gemme lister. Login huskes, indtil husets kode ændres eller appens data slettes.")).setPositiveButton("OK",null).show();
        }).setNegativeButton(I18n.text("Luk"),null).show();
    }
    @Override protected void onResume() { super.onResume();resumed=true; voiceHandler.removeCallbacks(voiceStatus); voiceHandler.post(voiceStatus); immersive(); if(web!=null) { web.onResume(); web.evaluateJavascript("window.dispatchEvent(new Event('online'))",null); } }
    @Override protected void onPause() {resumed=false; voiceHandler.removeCallbacks(voiceStatus);voiceHandler.removeCallbacks(pushVoice); if(web!=null) { CookieManager.getInstance().flush(); web.onPause(); } super.onPause(); }
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
    @Override protected void onDestroy() {if(voiceHost.get()==this)voiceHost.clear(); voiceHandler.removeCallbacksAndMessages(null); if(web!=null) web.destroy(); super.onDestroy(); }
}
