package app.foodie.mobile;

import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ApplicationInfo;
import android.content.pm.ResolveInfo;
import android.content.pm.ServiceInfo;
import android.provider.Settings;
import android.speech.RecognitionService;
import android.speech.SpeechRecognizer;
import java.util.ArrayList;
import java.util.List;

final class SpeechProviders {
    static List<ComponentName> available(Context context) {
        ComponentName selected = null;
        try {
            String value = Settings.Secure.getString(context.getContentResolver(), "voice_recognition_service");
            if (value != null) selected = ComponentName.unflattenFromString(value);
        } catch (RuntimeException ignored) {}
        final ComponentName systemDefault = selected;
        final String working = context.getSharedPreferences("kitchen", Context.MODE_PRIVATE).getString("workingRecognizer", "");
        List<ComponentName> candidates = new ArrayList<>();
        for (ResolveInfo info : context.getPackageManager().queryIntentServices(new Intent(RecognitionService.SERVICE_INTERFACE), 0)) {
            ServiceInfo service = info.serviceInfo;
            if (service == null || !service.enabled || !service.exported || !service.applicationInfo.enabled) continue;
            ComponentName name = new ComponentName(service.packageName, service.name);
            boolean google = service.packageName.equals("com.google.android.googlequicksearchbox") || service.packageName.equals("com.google.android.tts");
            boolean samsung = (service.applicationInfo.flags & ApplicationInfo.FLAG_SYSTEM) != 0 && (service.packageName.startsWith("com.samsung.") || service.packageName.startsWith("com.sec."));
            // Only the user's default and installed Google/Samsung providers;
            // do not send speech to an arbitrary third-party recognizer.
            if ((name.equals(systemDefault) || google || samsung) && !candidates.contains(name)) candidates.add(name);
        }
        java.util.Collections.sort(candidates, (a, b) -> Integer.compare(
            SpeechPolicy.priority(a.getPackageName(), a.equals(systemDefault), a.flattenToString().equals(working)),
            SpeechPolicy.priority(b.getPackageName(), b.equals(systemDefault), b.flattenToString().equals(working))));
        if (candidates.size() > 3) candidates = new ArrayList<>(candidates.subList(0, 3));
        if (systemDefault == null && candidates.size() < 3 && SpeechRecognizer.isRecognitionAvailable(context)) candidates.add(working.equals("default") ? 0 : candidates.size(), null);
        return candidates;
    }

    static String id(ComponentName name) { return name == null ? "default" : name.flattenToString(); }
}
