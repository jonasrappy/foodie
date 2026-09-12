package app.foodie.mobile;

public final class VoicePresentationTests {
    private static void check(boolean value,String message) { if(!value)throw new AssertionError(message); }
    public static void main(String[] args) {
        VoicePresentation scene=new VoicePresentation();
        scene.phase("speaking","Foodie er klar");
        check(!scene.snapshot().visible,"service startup must not open the conversation");
        scene.begin();long session=scene.snapshot().session;
        check(scene.snapshot().visible&&scene.snapshot().phase.equals("waking"),"wake opens the avatar");
        scene.phase("thinking","Hej");
        check(!scene.snapshot().phase.equals("speaking"),"queued TTS must not animate a speaking mouth");
        scene.phase("speaking","Hey. Hvad skal jeg tilføje?");scene.microphone(12);
        check(scene.snapshot().level==0,"microphone must not animate Foodie's own speech");
        scene.phase("listening","");scene.microphone(4);
        check(scene.snapshot().level==.5f,"listening level must reflect RMS");
        scene.phase("hearing","");scene.microphone(100);
        check(scene.snapshot().level==1,"microphone animation must be bounded");
        scene.microphone(Float.NaN);check(scene.snapshot().level==0,"invalid RMS must not corrupt the visual");
        scene.phase("thinking","");check(scene.snapshot().level==0,"processing clears the microphone animation");
        scene.end();scene.phase("speaking","stale");scene.microphone(10);
        check(!scene.snapshot().visible&&scene.snapshot().text.isEmpty(),"finished dialogue must stay closed on stale presentation updates");
        scene.begin();check(scene.snapshot().session>session,"each wake must identify a new conversation");
        System.out.println("PASS: wake-only presentation, playback/listening separation, bounded RMS, shutdown and session identity");
    }
}
