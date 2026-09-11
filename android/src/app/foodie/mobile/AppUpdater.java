package app.foodie.mobile;

import android.app.Activity;
import android.app.DownloadManager;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.Handler;
import android.os.Looper;
import android.provider.Settings;
import android.widget.Toast;
import java.io.File;
import java.util.Arrays;

/** User-started APK updates. Android owns the download and installation prompt. */
final class AppUpdater {
    static final int INSTALL_PERMISSION = 45;
    private final Activity activity;
    private final SharedPreferences prefs;
    private final DownloadManager downloads;
    private final Handler handler = new Handler(Looper.getMainLooper());
    private boolean resumed, checking;
    private final Runnable poll = () -> checkDownload();

    AppUpdater(Activity activity, SharedPreferences prefs) {
        this.activity = activity; this.prefs = prefs;
        downloads = (DownloadManager) activity.getSystemService(Activity.DOWNLOAD_SERVICE);
    }
    void resume() {
        long pending = prefs.getLong("updateDownload", -1);
        if (pending != -1 && prefs.getInt("updateFromCode", BuildConfig.VERSION_CODE) < BuildConfig.VERSION_CODE) clear(pending);
        resumed = true; handler.removeCallbacks(poll); handler.post(poll); }
    void pause() { resumed = false; handler.removeCallbacks(poll); }
    void start(Uri uri) {
        // The Activity additionally checks the configured HTTPS origin and page.
        if (!"/api/download/android".equals(uri.getPath()) || uri.getQueryParameter("grant") == null) return;
        long pending = prefs.getLong("updateDownload", -1);
        if (pending != -1) {
            prefs.edit().putBoolean("updatePresented", false).apply();
            checkDownload(); return;
        }
        try {
            String name = "foodie-" + System.currentTimeMillis() + ".apk";
            DownloadManager.Request request = new DownloadManager.Request(uri)
                .setTitle("Foodie").setDescription(I18n.text("Henter opdatering …"))
                .setMimeType("application/vnd.android.package-archive")
                .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                .setDestinationInExternalFilesDir(activity, Environment.DIRECTORY_DOWNLOADS, name);
            long id = downloads.enqueue(request);
            prefs.edit().putLong("updateDownload", id).putString("updateFile", name).putInt("updateFromCode", BuildConfig.VERSION_CODE).putBoolean("updatePresented", false).apply();
            message("Henter opdatering …");
            handler.removeCallbacks(poll); handler.post(poll);
        } catch (RuntimeException error) { message("Kunne ikke hente opdateringen. Tryk Opdater for at prøve igen."); }
    }
    private void checkDownload() {
        if (!resumed || checking) return;
        long id = prefs.getLong("updateDownload", -1);
        if (id == -1) return;
        checking = true;
        try (Cursor result = downloads.query(new DownloadManager.Query().setFilterById(id))) {
            if (result == null || !result.moveToFirst()) { clear(id); return; }
            int status = result.getInt(result.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS));
            if (status == DownloadManager.STATUS_FAILED) {
                clear(id); message("Kunne ikke hente opdateringen. Tryk Opdater for at prøve igen.");
            } else if (status == DownloadManager.STATUS_SUCCESSFUL) {
                if (!prefs.getBoolean("updatePresented", false)) install(id);
            } else {
                handler.removeCallbacks(poll); handler.postDelayed(poll, 1500);
            }
        } catch (RuntimeException error) {
            message("Kunne ikke åbne opdateringen. Tryk Opdater for at prøve igen.");
        } finally { checking = false; }
    }
    @SuppressWarnings("deprecation")
    private void install(long id) {
        try {
            String name = prefs.getString("updateFile", "");
            if (!name.matches("foodie-[0-9]+\\.apk")) throw new IllegalStateException();
            File directory = activity.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS);
            if (directory == null) throw new IllegalStateException();
            PackageManager pm = activity.getPackageManager();
            PackageInfo candidate = pm.getPackageArchiveInfo(new File(directory, name).getAbsolutePath(), PackageManager.GET_SIGNATURES);
            PackageInfo installed = pm.getPackageInfo(activity.getPackageName(), PackageManager.GET_SIGNATURES);
            if (candidate == null || !activity.getPackageName().equals(candidate.packageName) || !sameSignatures(candidate.signatures, installed.signatures)) {
                clear(id); message("Opdateringen passer ikke til denne app. Hent en APK med samme app-id og signeringsnøgle."); return;
            }
            if (candidate.versionCode <= installed.versionCode) { clear(id); return; }
            prefs.edit().putBoolean("updatePresented", true).apply();
            if (Build.VERSION.SDK_INT >= 26 && !pm.canRequestPackageInstalls()) {
                message("Tillad opdateringer fra Foodie på den næste skærm.");
                activity.startActivityForResult(new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:" + activity.getPackageName())), INSTALL_PERMISSION);
                return;
            }
            Uri apk = downloads.getUriForDownloadedFile(id);
            if (apk == null || !"content".equals(apk.getScheme())) throw new IllegalStateException();
            Intent intent = new Intent(Intent.ACTION_VIEW).setDataAndType(apk, "application/vnd.android.package-archive")
                .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            activity.startActivity(intent);
        } catch (Exception error) {
            prefs.edit().putBoolean("updatePresented", true).apply();
            message("Kunne ikke åbne opdateringen. Tryk Opdater for at prøve igen.");
        }
    }
    void permissionReturned() {
        if (Build.VERSION.SDK_INT < 26 || activity.getPackageManager().canRequestPackageInstalls()) {
            prefs.edit().putBoolean("updatePresented", false).apply();
            handler.post(poll);
        }
    }
    private static boolean sameSignatures(Signature[] a, Signature[] b) {
        if (a == null || b == null || a.length == 0 || a.length != b.length) return false;
        for (Signature signature : a) if (!Arrays.asList(b).contains(signature)) return false;
        return true;
    }
    private void clear(long id) {
        downloads.remove(id);
        prefs.edit().remove("updateDownload").remove("updateFile").remove("updateFromCode").remove("updatePresented").apply();
    }
    private void message(String text) { Toast.makeText(activity, I18n.text(text), Toast.LENGTH_LONG).show(); }
}
