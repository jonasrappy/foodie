package app.foodie.mobile;

/** Bounded recovery for Android recognition failures. Never changes the language. */
final class SpeechPolicy {
    enum Action { RETRY, NEXT, STOP }
    private final int providers;
    private int index, attempts = 1;
    private boolean retried;

    SpeechPolicy(int providers) { this.providers = providers; }
    int index() { return index; }

    Action recover(int code) {
        // Silence/unclear speech is not a broken provider. Permission denials
        // must not trigger another capture attempt either.
        if (code == 6 || code == 7 || code == 9 || attempts >= 4) return Action.STOP;
        if ((code == 3 || code == 5 || code == 8 || code == 11 || code == 100 || code == 101) && !retried) {
            retried = true; attempts++; return Action.RETRY;
        }
        if (index + 1 < providers) {
            index++; retried = false; attempts++; return Action.NEXT;
        }
        if (!retried && (code == 1 || code == 2 || code == 4)) {
            retried = true; attempts++; return Action.RETRY;
        }
        return Action.STOP;
    }

    static int priority(String packageName, boolean isDefault, boolean succeeded) {
        if (succeeded) return 0;
        if (packageName.equals("com.google.android.googlequicksearchbox")) return 10;
        if (packageName.equals("com.google.android.tts")) return 20;
        return isDefault ? 30 : 40;
    }

    static String message(int code) {
        switch (code) {
            case 1: case 2:
                return "Androids taletjeneste kunne ikke få forbindelse. Prøv igen om lidt.";
            case 3:
                return "Taletjenesten kunne ikke åbne mikrofonen. Luk andre apps, der bruger den, og prøv igen.";
            case 6: case 7:
                return "Jeg hørte ikke et klart svar. Sig Hey Foodie for at prøve igen.";
            case 9:
                return "Androids taletjeneste fik ikke adgang til mikrofonen. Kontrollér mikrofontilladelserne i Android.";
            case 12: case 13:
                return "Dansk talegenkendelse er ikke tilgængelig. Åbn mikrofonens indstillinger, og kontrollér taletjenesten.";
            case 8: case 10:
                return "Androids taletjeneste er optaget. Prøv igen om lidt.";
            default:
                return "Androids taletjeneste fejlede. Den præcise fejl står under mikrofonens indstillinger.";
        }
    }
}
