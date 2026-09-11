package app.foodie.mobile;
/** PCM format used only by the local wake-word detector. */
final class VoiceAudio {
 static final int RATE=16000,FRAME=1600;
 static double rms(short[] samples,int count){double sum=0;for(int i=0;i<count;i++)sum+=(double)samples[i]*samples[i];return Math.sqrt(sum/Math.max(1,count));}
}
