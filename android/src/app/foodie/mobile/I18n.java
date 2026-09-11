package app.foodie.mobile;

import android.content.Context;
import org.json.JSONObject;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;

/** Read-only language pack selected from FOODIE_LANGUAGE at build time. */
final class I18n {
    private static JSONObject messages;
    private static String language="en",locale="en-US";
    static synchronized void init(Context context) {
        if(messages!=null)return;
        try(InputStream input=context.getAssets().open("i18n.json")) {
            ByteArrayOutputStream output=new ByteArrayOutputStream();byte[] buffer=new byte[4096];int n;
            while((n=input.read(buffer))!=-1)output.write(buffer,0,n);
            JSONObject pack=new JSONObject(new String(output.toByteArray(),StandardCharsets.UTF_8));
            language=pack.getString("language");locale=pack.getString("locale");messages=pack.getJSONObject("messages");
        } catch(Exception error) {throw new IllegalStateException("The APK is missing its language pack",error);}
    }
    static String text(String source) {return messages==null?source:messages.optString(source,source);}
    static String language(){return language;}
    static String locale(){return locale;}
}
