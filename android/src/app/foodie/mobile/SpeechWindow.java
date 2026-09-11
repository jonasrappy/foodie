package app.foodie.mobile;

/** Five seconds to begin a follow-up; a started sentence may finish normally. */
final class SpeechWindow {
    static final long FOLLOW_UP_MS = 5000;
    private final boolean optional;
    private final long deadline;
    private boolean speech, ended;

    SpeechWindow(boolean optional, long now) {
        this.optional = optional;
        deadline = now + FOLLOW_UP_MS;
    }

    boolean expired(long now) { return optional && !speech && now >= deadline; }
    boolean begin(long now) {
        if (ended || expired(now)) return false;
        speech = true;
        return true;
    }
    boolean finish(long now) {
        if (ended) return false;
        ended = true;
        return !expired(now);
    }
    boolean closeIfSilent(long now) {
        if (ended || !expired(now)) return false;
        ended = true;
        return true;
    }
}
