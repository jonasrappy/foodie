package app.foodie.mobile;

/** Presentation only: never parses commands or changes recognition policy. */
final class VoicePresentation {
    static final class Frame {
        final boolean visible;
        final long session;
        final String phase, text;
        final float level;
        Frame(boolean visible, long session, String phase, String text, float level) {
            this.visible=visible; this.session=session; this.phase=phase; this.text=text; this.level=level;
        }
    }
    private Frame frame=new Frame(false,0,"idle","",0);
    synchronized Frame snapshot() { return frame; }
    synchronized void begin() { frame=new Frame(true,frame.session+1,"waking","Hey Foodie",0); }
    synchronized void phase(String phase,String text) {
        if(frame.visible)frame=new Frame(true,frame.session,phase,text,0);
    }
    synchronized void microphone(float db) {
        if(!frame.visible||!(frame.phase.equals("listening")||frame.phase.equals("hearing")))return;
        float level=Float.isNaN(db)||Float.isInfinite(db)?0:Math.max(0,Math.min(1,(db+2)/12));
        frame=new Frame(true,frame.session,frame.phase,frame.text,level);
    }
    synchronized void end() { frame=new Frame(false,frame.session,"idle","",0); }
}
