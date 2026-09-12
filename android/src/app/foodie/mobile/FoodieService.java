package app.foodie.mobile;

import android.Manifest;
import android.widget.Toast;
import android.app.*;
import android.content.*;
import android.content.pm.PackageManager;
import android.content.pm.ServiceInfo;
import android.media.*;
import android.os.*;
import android.speech.tts.*;
import android.speech.SpeechRecognizer;
import android.speech.RecognitionListener;
import android.speech.RecognizerIntent;
import org.json.JSONObject;
import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;

/** User-started microphone foreground service. Lives independently of the WebView/screen. */
public final class FoodieService extends Service implements TextToSpeech.OnInitListener {
 public static final String START="app.foodie.mobile.FOODIE_START",STOP="app.foodie.mobile.FOODIE_STOP",DISMISS="app.foodie.mobile.FOODIE_DISMISS";
 static final VoicePresentation presentation=new VoicePresentation();
 public static String greeting(){return I18n.text("Hey. What should I add to the shopping list?");}
 public static volatile boolean running=false;
 public static volatile String status=I18n.text("Off");
 private static final String CHANNEL="foodie-microphone";
 private static final int NOTICE=41;
 private final Handler main=new Handler(Looper.getMainLooper());
 private final ExecutorService worker=Executors.newSingleThreadExecutor();
 private final ExecutorService diagnosticWorker=Executors.newSingleThreadExecutor();
 private int diagnosticBudget=15;
 private final AtomicInteger generation=new AtomicInteger();
 private volatile AudioRecord recorder;
 private volatile HttpURLConnection connection;
 private volatile boolean active;
 private FoodieWakeWord wakeWord;
 private TextToSpeech speaker;
 private SpeechRecognizer recognizer;
 private List<ComponentName> speechProviders;
 private SpeechPolicy speechPolicy;
 private PowerManager.WakeLock cpu;
 private boolean modelReady,voiceReady,introduced;
 private String token,speechID;
 private Runnable afterSpeech;
 private double ambient=75;
 private int turns;
 private final Runnable renewWake=new Runnable(){public void run(){if(active&&cpu!=null){if(cpu.isHeld())cpu.release();cpu.acquire(2*60*60*1000L);main.postDelayed(this,60*60*1000L);}}};
 @Override public void onCreate(){super.onCreate();I18n.init(this);status=I18n.text(status);if(Build.VERSION.SDK_INT>=26){NotificationChannel channel=new NotificationChannel(CHANNEL,I18n.text("Hey Foodie is listening"),NotificationManager.IMPORTANCE_LOW);channel.setDescription(I18n.text("Voice control with the screen off"));channel.setShowBadge(false);getSystemService(NotificationManager.class).createNotificationChannel(channel);}}
 @Override public int onStartCommand(Intent intent,int flags,int startId){
  if(intent==null||STOP.equals(intent.getAction())){stopVoice();return START_NOT_STICKY;}
  if(DISMISS.equals(intent.getAction())){if(active)cancelConversation();else stopSelf();return START_NOT_STICKY;}
  if(active)return START_NOT_STICKY;
  token=getSharedPreferences("kitchen",MODE_PRIVATE).getString("voiceToken","");
  if(checkSelfPermission(Manifest.permission.RECORD_AUDIO)!=PackageManager.PERMISSION_GRANTED||!token.startsWith("device.")){fail(I18n.text("Open the app, log in and allow microphone access."));return START_NOT_STICKY;}
  try {
   active=true;running=true;status=I18n.text("Starting Foodie …");
   if(Build.VERSION.SDK_INT>=29)startForeground(NOTICE,notification(),ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE);else startForeground(NOTICE,notification());
   cpu=((PowerManager)getSystemService(POWER_SERVICE)).newWakeLock(PowerManager.PARTIAL_WAKE_LOCK,"mad:foodie");cpu.setReferenceCounted(false);renewWake.run();
   speaker=new TextToSpeech(this,this);
   worker.execute(()->{try{wakeWord=new FoodieWakeWord(this);main.post(()->{modelReady=true;ready();});}catch(Throwable error){main.post(()->fail(I18n.text("Foodie couldn't start the voice model. Open the app and try again.")));}});
  }catch(Exception error){fail(I18n.text("Android couldn't start the microphone. Open the app and try again."));}
  return START_NOT_STICKY;
 }
 @Override public void onInit(int result){
  if(!active)return;
  if(result!=TextToSpeech.SUCCESS){fail(I18n.text("Speech could not start. Check Android's text-to-speech settings."));return;}
  int language=speaker.setLanguage(Locale.forLanguageTag(I18n.locale()));
  if(language==TextToSpeech.LANG_MISSING_DATA||language==TextToSpeech.LANG_NOT_SUPPORTED){fail(I18n.text("The selected voice is missing. Install it in Android's text-to-speech settings."));return;}
  // Prefer an installed voice in the selected language so replies work without a third-party server.
  Set<Voice> voices=speaker.getVoices();if(voices!=null)for(Voice voice:voices)if(I18n.language().equals(voice.getLocale().getLanguage())&&!voice.isNetworkConnectionRequired()){speaker.setVoice(voice);break;}
  speaker.setSpeechRate(.98f);speaker.setAudioAttributes(new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_ASSISTANT).setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build());
  speaker.setOnUtteranceProgressListener(new UtteranceProgressListener(){
   public void onStart(String id){main.post(()->{if(active&&id.equals(speechID))scene("speaking",status);});}
   public void onDone(String id){main.post(()->{if(active&&id.equals(speechID)){scene("thinking","");Runnable next=afterSpeech;afterSpeech=null;main.postDelayed(()->{if(active&&id.equals(speechID)&&next!=null)next.run();},350);}});}
   public void onStop(String id,boolean interrupted){main.post(()->{if(active&&id.equals(speechID)){speechID=null;afterSpeech=null;listenWake();}});}
   public void onError(String id){main.post(()->{if(active&&id.equals(speechID))fail(I18n.text("Speech playback failed. Check the voice and media volume in Android."));});}
  });voiceReady=true;ready();
 }
 private void ready(){if(active&&modelReady&&voiceReady&&!introduced){introduced=true;say(I18n.text("Foodie is ready. Say Hey Foodie when you'd like to add something."),this::listenWake);}}
 private void setStatus(String text){status=text;MainActivity.voiceChanged();if(active)((NotificationManager)getSystemService(NOTIFICATION_SERVICE)).notify(NOTICE,notification());}
 private void scene(String phase,String text){presentation.phase(phase,text);MainActivity.voiceChanged();}
 private void wakeConversation(){
  presentation.begin();MainActivity.voiceChanged();
  try{startActivity(new Intent(this,MainActivity.class).setAction(MainActivity.VOICE_WAKE).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK|Intent.FLAG_ACTIVITY_SINGLE_TOP));}
  catch(RuntimeException error){getSharedPreferences("kitchen",MODE_PRIVATE).edit().putString("voiceScreenError",I18n.text("Foodie couldn't open. Allow Display over other apps in the screen wake settings.")).apply();}
 }
 private void cancelConversation(){
  speechID=null;afterSpeech=null;if(speaker!=null)speaker.stop();
  HttpURLConnection http=connection;if(http!=null)diagnosticWorker.execute(http::disconnect);
  listenWake();
 }
 private Notification notification(){
  PendingIntent open=PendingIntent.getActivity(this,0,new Intent(this,MainActivity.class),PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);
  PendingIntent stop=PendingIntent.getService(this,1,new Intent(this,FoodieService.class).setAction(STOP),PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);
  Notification.Builder builder=Build.VERSION.SDK_INT>=26?new Notification.Builder(this,CHANNEL):new Notification.Builder(this);
  return builder.setSmallIcon(R.drawable.ic_foodie).setContentTitle("Hey Foodie").setContentText(status).setContentIntent(open).setOngoing(true).setOnlyAlertOnce(true).setCategory(Notification.CATEGORY_SERVICE).setVisibility(Notification.VISIBILITY_PUBLIC).addAction(new Notification.Action.Builder(null,I18n.text("Turn off"),stop).build()).build();
 }
 private void stopCapture(){generation.incrementAndGet();SpeechRecognizer old=recognizer;recognizer=null;if(old!=null){try{old.cancel();}catch(RuntimeException ignored){}try{old.destroy();}catch(RuntimeException ignored){}}AudioRecord current=recorder;if(current!=null)try{current.stop();}catch(IllegalStateException ignored){}}
 private void say(String text,Runnable next){
  if(!active)return;text=I18n.text(text);stopCapture();setStatus(text);scene("thinking",text);speechID=UUID.randomUUID().toString();afterSpeech=next;
  Bundle options=new Bundle();options.putFloat(TextToSpeech.Engine.KEY_PARAM_VOLUME,1f);
  if(speaker.speak(text,TextToSpeech.QUEUE_FLUSH,options,speechID)==TextToSpeech.ERROR){fail(I18n.text("Speech could not be played."));return;}
  final String expected=speechID;main.postDelayed(()->{if(active&&expected.equals(speechID)&&afterSpeech!=null){fail(I18n.text("Speech playback didn't respond. Check Android's text-to-speech settings."));}},30000);
 }
 private AudioRecord openRecorder(){
  int minimum=AudioRecord.getMinBufferSize(VoiceAudio.RATE,AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT);
  AudioRecord audio=new AudioRecord(MediaRecorder.AudioSource.VOICE_RECOGNITION,VoiceAudio.RATE,AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT,Math.max(12800,minimum*2));
  if(audio.getState()!=AudioRecord.STATE_INITIALIZED){audio.release();throw new IllegalStateException("microphone unavailable");}
  recorder=audio;audio.startRecording();if(audio.getRecordingState()!=AudioRecord.RECORDSTATE_RECORDING){audio.release();recorder=null;throw new IllegalStateException("recording unavailable");}return audio;
 }
 private void closeRecorder(AudioRecord audio){if(audio!=null){try{audio.stop();}catch(IllegalStateException ignored){}audio.release();if(recorder==audio)recorder=null;}}
 private void listenWake(){
  if(!active)return;presentation.end();stopCapture();setStatus(I18n.text("Listening for Hey Foodie"));final int epoch=generation.get();
  worker.execute(()->{AudioRecord audio=null;boolean detected=false;try{
   if(!active||epoch!=generation.get())return;wakeWord.reset();audio=openRecorder();short[] pcm=new short[VoiceAudio.FRAME];float[] samples=new float[VoiceAudio.FRAME];
   while(active&&epoch==generation.get()){
    int n=audio.read(pcm,0,pcm.length,AudioRecord.READ_BLOCKING);if(n<=0){if(active&&epoch==generation.get())throw new IOException("microphone stopped");break;}
    double level=VoiceAudio.rms(pcm,n);if(level<700)ambient=ambient*.995+level*.005;
    for(int i=0;i<n;i++)samples[i]=pcm[i]/32768f;
    if(wakeWord.accept(n==samples.length?samples:Arrays.copyOf(samples,n))){detected=true;break;}
   }
  }catch(Exception error){if(active&&epoch==generation.get())main.post(()->{if(active&&epoch==generation.get())fail(I18n.text("The microphone is busy or turned off. Open the app and try again."));});}
  finally{closeRecorder(audio);}
  if(detected&&active&&epoch==generation.get())main.post(()->{if(active&&epoch==generation.get()){turns=0;wakeConversation();say(greeting(),()->listenCommand(""));}});
  });
 }
 private void listenCommand(String confirmation){
  listenCommand(confirmation,false);
 }
 private void listenCommand(String confirmation,boolean followUp){
  if(!active)return;if(++turns>4){say(I18n.text("Say Hey Foodie to try again."),this::listenWake);return;}
  try{speechProviders=SpeechProviders.available(this);}
  catch(RuntimeException error){speechProviders=new ArrayList<>();if(SpeechRecognizer.isRecognitionAvailable(this))speechProviders.add(null);}
  if(speechProviders.isEmpty()){say(I18n.text("English speech recognition is missing. Enable the Google app in Android's app settings."),this::listenWake);return;}
  speechPolicy=new SpeechPolicy(speechProviders.size());
  listenAndroid(confirmation,followUp);
 }
 private void recognitionFailed(int code,String phase,String confirmation,ComponentName provider,boolean ready,boolean heard,boolean followUp){
  if(!active)return;
  // Silence after I18n.text("Anything else?") is a normal end of the conversation.
  if(followUp&&(code==SpeechRecognizer.ERROR_SPEECH_TIMEOUT||code==SpeechRecognizer.ERROR_NO_MATCH)){listenWake();return;}
  recordSpeechFailure(code,phase,provider,ready,heard);
  SpeechPolicy.Action action=speechPolicy.recover(code);
  if(action!=SpeechPolicy.Action.STOP){
   String prompt=confirmation.isEmpty()?I18n.text("I'll try speech recognition again. Please repeat the item."):I18n.text("I'll try speech recognition again. Please say yes or no.");
   say(prompt,()->listenAndroid(confirmation,followUp));
  }else say(SpeechPolicy.message(code),this::listenWake);
 }
 private void listenAndroid(String confirmation,boolean followUp){
  if(!active)return;
  stopCapture();final int epoch=generation.get();final boolean[] completed={false},ready={false},heard={false};
  final SpeechWindow window=new SpeechWindow(followUp,SystemClock.elapsedRealtime());
  final ComponentName provider=speechProviders.get(speechPolicy.index());
  setStatus(I18n.text("Starting speech recognition …"));
  scene("thinking","");
  if(followUp)main.postDelayed(()->{
   if(active&&epoch==generation.get()&&!completed[0]&&window.closeIfSilent(SystemClock.elapsedRealtime())){
    completed[0]=true;listenWake();
   }
  },SpeechWindow.FOLLOW_UP_MS);
  try{
   recognizer=provider==null?SpeechRecognizer.createSpeechRecognizer(this):SpeechRecognizer.createSpeechRecognizer(this,provider);
   recognizer.setRecognitionListener(new RecognitionListener(){
    private boolean current(){return active&&epoch==generation.get();}
    public void onReadyForSpeech(Bundle parameters){
     if(!current()||completed[0])return;ready[0]=true;
     setStatus(confirmation.isEmpty()?I18n.text("Listening for your item …"):I18n.text("Waiting for yes or no …"));
     scene("listening",confirmation.isEmpty()?(followUp?I18n.text("Say an item or no thanks"):I18n.text("What should go on the list?")):I18n.text("Say yes or no"));
     main.postDelayed(()->{if(current()&&!completed[0]){completed[0]=true;recognitionFailed(heard[0]?101:SpeechRecognizer.ERROR_SPEECH_TIMEOUT,"result_timeout",confirmation,provider,true,heard[0],followUp);}},20000);
    }
    public void onBeginningOfSpeech(){
     if(!current()||completed[0])return;
     if(window.begin(SystemClock.elapsedRealtime())){heard[0]=true;scene("hearing","");}
     else{completed[0]=true;listenWake();}
    }
    public void onRmsChanged(float rms){if(current()&&!completed[0]){presentation.microphone(rms);MainActivity.voiceChanged();}}
    public void onBufferReceived(byte[] buffer){}
    public void onEndOfSpeech(){if(current()&&!completed[0]){setStatus(I18n.text("Understanding your request …"));scene("thinking","");}}
    public void onPartialResults(Bundle partial){}
    public void onEvent(int type,Bundle parameters){}
    public void onError(int code){
     if(!current()||completed[0])return;completed[0]=true;
     if(window.expired(SystemClock.elapsedRealtime())){listenWake();return;}
     recognitionFailed(code,"recognition",confirmation,provider,ready[0],heard[0],followUp);
    }
    public void onResults(Bundle result){
     if(!current()||completed[0])return;completed[0]=true;
     // Reject results arriving after a silent deadline, even if Android's
     // timer callback was delayed. A sentence started in time may finish.
     if(!window.finish(SystemClock.elapsedRealtime())){listenWake();return;}
     ArrayList<String> candidates=result==null?null:result.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);
     float[] confidence=result==null?null:result.getFloatArray(SpeechRecognizer.CONFIDENCE_SCORES);
     if(candidates==null||candidates.isEmpty()||candidates.get(0)==null||candidates.get(0).trim().isEmpty()||(confidence!=null&&confidence.length>0&&confidence[0]>=0&&confidence[0]<.35f)){
      if(followUp&&!heard[0])listenWake();else say(I18n.text("I'm not sure what that item was. Please try again."),()->listenCommand(confirmation,followUp));return;
     }
     getSharedPreferences("kitchen",MODE_PRIVATE).edit().putString("workingRecognizer",SpeechProviders.id(provider)).apply();
     try{JSONObject payload=new JSONObject();payload.put("text",candidates.get(0));byte[] bytes=payload.toString().getBytes(StandardCharsets.UTF_8);setStatus(I18n.text("Saving to the shopping list …"));scene("thinking",candidates.get(0));worker.execute(()->submit(bytes,confirmation,epoch,followUp));}
     catch(Exception error){say(I18n.text("I couldn't understand the item. Please try again."),FoodieService.this::listenWake);}
    }
   });
   // Use the configured recognition language across Google and Samsung providers.
   // Optional recognition hints are omitted for compatibility; the exact
   // provider error is recorded separately instead of guessing its cause.
   Intent request=new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH);
   request.putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL,RecognizerIntent.LANGUAGE_MODEL_FREE_FORM);
   request.putExtra(RecognizerIntent.EXTRA_LANGUAGE,I18n.locale());
   request.putExtra(RecognizerIntent.EXTRA_MAX_RESULTS,3);
   request.putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS,false);
   recognizer.startListening(request);
  }catch(RuntimeException error){
   completed[0]=true;recognitionFailed(error instanceof SecurityException?SpeechRecognizer.ERROR_INSUFFICIENT_PERMISSIONS:SpeechRecognizer.ERROR_CLIENT,"start",confirmation,provider,false,false,followUp);return;
  }
  main.postDelayed(()->{if(active&&epoch==generation.get()&&!completed[0]&&!ready[0]){completed[0]=true;recognitionFailed(100,"readiness_timeout",confirmation,provider,false,heard[0],followUp);}},8000);
 }
 private void recordSpeechFailure(int code,String phase,ComponentName provider,boolean ready,boolean heard){
  final String name=SpeechProviders.id(provider);
  String detail="APK "+BuildConfig.VERSION_NAME+" · Android "+Build.VERSION.SDK_INT+I18n.text("\nSpeech service: ")+name+I18n.text("\nError: ")+code+" · "+phase+I18n.text("\nReady: ")+ready+I18n.text(" · Speech detected: ")+heard;
  getSharedPreferences("kitchen",MODE_PRIVATE).edit().putString("voiceDiagnostic",detail).apply();
  android.util.Log.w("FoodieSpeech",detail);
  if(diagnosticBudget--<=0)return;
  try{
   JSONObject event=new JSONObject();event.put("version",BuildConfig.VERSION_NAME);event.put("sdk",Build.VERSION.SDK_INT);event.put("provider",name);event.put("phase",phase);event.put("code",code);event.put("ready",ready);event.put("heard",heard);
   final byte[] data=event.toString().getBytes(StandardCharsets.UTF_8);final String deviceToken=token;
   diagnosticWorker.execute(()->{
    HttpURLConnection http=null;
    try{
     http=(HttpURLConnection)new URL(BuildConfig.SITE_URL+"api/voice/diagnostic").openConnection();
     http.setInstanceFollowRedirects(false);http.setConnectTimeout(3000);http.setReadTimeout(3000);http.setRequestMethod("POST");http.setDoOutput(true);http.setFixedLengthStreamingMode(data.length);
     http.setRequestProperty("Authorization","Bearer "+deviceToken);http.setRequestProperty("Content-Type","application/json");
     try(OutputStream out=http.getOutputStream()){out.write(data);}http.getResponseCode();
    }catch(Exception ignored){}finally{if(http!=null)http.disconnect();}
   });
  }catch(Exception ignored){}
 }
 private void submit(byte[] payload,String confirmation,int epoch,boolean followUp){
  String requestID=UUID.randomUUID().toString();
  for(int attempt=0;attempt<2&&active&&epoch==generation.get();attempt++){
   HttpURLConnection http=null;
   try{
    http=(HttpURLConnection)new URL(BuildConfig.SITE_URL+"api/voice/text").openConnection();connection=http;http.setInstanceFollowRedirects(false);http.setConnectTimeout(8000);http.setReadTimeout(80000);http.setRequestMethod("POST");http.setDoOutput(true);http.setFixedLengthStreamingMode(payload.length);http.setRequestProperty("Content-Type","application/json");http.setRequestProperty("Authorization","Bearer "+token);http.setRequestProperty("X-Request-ID",requestID);if(!confirmation.isEmpty())http.setRequestProperty("X-Voice-Confirmation",confirmation);
    try(OutputStream out=http.getOutputStream()){out.write(payload);}
    int code=http.getResponseCode();InputStream source=code>=400?http.getErrorStream():http.getInputStream();String body=readResponse(source);JSONObject result=new JSONObject(body);
    if(!active||epoch!=generation.get())return;
    if(code==401){main.post(()->{if(active&&epoch==generation.get()){getSharedPreferences("kitchen",MODE_PRIVATE).edit().remove("voiceToken").apply();say(I18n.text("Log in again in the app so I can save to the shopping list."),this::stopVoice);}});return;}
    if(code!=200){String error=result.optString("error",I18n.text("I couldn't save the item. Try again shortly."));main.post(()->{if(active&&epoch==generation.get())say(error,this::listenWake);});return;}
    String kind=result.optString("kind"),speech=result.optString("speech",I18n.text("I couldn't understand the item."));
    if(speech.length()>700)throw new IOException("invalid speech response");
    final String id="confirm".equals(kind)?result.getString("confirmation_id"):"";
    main.post(()->{
     if(!active||epoch!=generation.get())return;
     if("confirm".equals(kind))say(speech,()->listenCommand(id,followUp));
     else if("retry".equals(kind))say(speech,()->listenCommand("",followUp));
     else if("added".equals(kind)){
      // Bound retries per item, while allowing any number of successful items.
      turns=0;say(speech+I18n.text(" Anything else?"),()->listenCommand("",true));
     }else if("cancelled".equals(kind))say("Ok",this::listenWake);
     else say(speech,this::listenWake);
    });return;
   }catch(Exception error){if(attempt==1&&active&&epoch==generation.get())main.post(()->{if(active&&epoch==generation.get())say(I18n.text("I couldn't confirm that it was saved. Check the shopping list and try again when you're connected."),this::listenWake);});}
   finally{if(http!=null)http.disconnect();if(connection==http)connection=null;}
  }
 }
 private String readResponse(InputStream source)throws IOException{
  if(source==null)throw new IOException("empty response");try(InputStream input=source;ByteArrayOutputStream output=new ByteArrayOutputStream()){byte[] buffer=new byte[1024];int count;while((count=input.read(buffer))!=-1){if(output.size()+count>8192)throw new IOException("response too large");output.write(buffer,0,count);}return new String(output.toByteArray(),StandardCharsets.UTF_8);}
 }
 private void fail(String message){status=message;getSharedPreferences("kitchen",MODE_PRIVATE).edit().putString("voiceError",message).apply();Toast.makeText(this,message,Toast.LENGTH_LONG).show();shutdown(false);}
 private void stopVoice(){status=I18n.text("Off");shutdown(true);}
 private void shutdown(boolean clearError){
  active=false;running=false;presentation.end();MainActivity.voiceChanged();stopCapture();HttpURLConnection http=connection;if(http!=null)http.disconnect();main.removeCallbacksAndMessages(null);
  if(clearError)getSharedPreferences("kitchen",MODE_PRIVATE).edit().remove("voiceError").apply();
  if(speaker!=null){speaker.stop();speaker.shutdown();speaker=null;}
  if(cpu!=null&&cpu.isHeld())cpu.release();stopForeground(true);stopSelf();
 }
 @Override public void onDestroy(){shutdown(false);diagnosticWorker.shutdownNow();worker.execute(()->{if(wakeWord!=null){wakeWord.close();wakeWord=null;}});worker.shutdown();super.onDestroy();}
 @Override public IBinder onBind(Intent intent){return null;}
}
