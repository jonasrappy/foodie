package app.foodie.mobile;

public final class SpeechPolicyTests {
    private static void check(boolean ok, String reason) { if (!ok) throw new AssertionError(reason); }
    public static void main(String[] args) {
        SpeechPolicy language = new SpeechPolicy(2);
        check(language.recover(12) == SpeechPolicy.Action.NEXT && language.index() == 1, "unsupported language must try the next provider");
        check(language.recover(13) == SpeechPolicy.Action.STOP, "missing language must stop, never switch languages");
        SpeechPolicy busy = new SpeechPolicy(2);
        check(busy.recover(8) == SpeechPolicy.Action.RETRY && busy.index() == 0, "busy provider gets one reconnect");
        check(busy.recover(8) == SpeechPolicy.Action.NEXT && busy.index() == 1, "repeated busy must switch provider");
        check(busy.recover(5) == SpeechPolicy.Action.RETRY, "client failure gets one reconnect");
        check(busy.recover(5) == SpeechPolicy.Action.STOP, "recovery must be bounded");
        for (int code : new int[]{6,7,9}) check(new SpeechPolicy(3).recover(code) == SpeechPolicy.Action.STOP, "silence or permissions must not switch services");
        SpeechPolicy network = new SpeechPolicy(1);
        check(network.recover(2) == SpeechPolicy.Action.RETRY && network.recover(2) == SpeechPolicy.Action.STOP, "network retries must stop");
        check(SpeechPolicy.priority("com.google.android.googlequicksearchbox", false, false) < SpeechPolicy.priority("com.samsung.svoice", true, false), "prefer Google when installed");
        check(SpeechPolicy.priority("com.samsung.svoice", true, true) < SpeechPolicy.priority("com.google.android.googlequicksearchbox", false, false), "remember a working service");
        check(!SpeechPolicy.message(12).contains("connect") && !SpeechPolicy.message(9).contains("connect"), "language/permission errors are not network failures");
        System.out.println("PASS: provider fallback, bounded reconnects, configured language handling, permission/silence handling and precise errors");
    }
}
