package app.foodie.mobile;

public final class SpeechWindowTests {
    private static void check(boolean ok, String reason) { if (!ok) throw new AssertionError(reason); }
    public static void main(String[] args) {
        SpeechWindow silent = new SpeechWindow(true, 1000);
        check(!silent.closeIfSilent(5999), "must allow all five seconds");
        check(silent.closeIfSilent(6000), "must stop at five seconds of silence");
        check(!silent.begin(6001) && !silent.finish(6001), "late callbacks must never add a grocery");
        check(!silent.closeIfSilent(7000), "timeout must fire at most once");
        SpeechWindow speaking = new SpeechWindow(true, 1000);
        check(speaking.begin(5999), "speech before the deadline must count");
        check(!speaking.closeIfSilent(6000), "do not interrupt a sentence already started");
        check(speaking.finish(12000), "allow recognition to finish after the input window");
        check(!speaking.finish(12001), "a final result must be handled only once");
        SpeechWindow delayed = new SpeechWindow(true, 1000);
        check(!delayed.begin(6500) && !delayed.finish(6500), "a delayed timer must not admit late speech");
        SpeechWindow ordinary = new SpeechWindow(false, 1000);
        check(!ordinary.closeIfSilent(7000) && ordinary.begin(7000) && ordinary.finish(15000), "keep the initial command's normal timeout");
        System.out.println("PASS: five-second idle window, full spoken sentences, late-result rejection and unchanged initial listening");
    }
}
