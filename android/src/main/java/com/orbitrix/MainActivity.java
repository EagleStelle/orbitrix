package com.orbitrix;

import android.app.Activity;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.pm.ActivityInfo;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Bundle;
import android.provider.Settings;
import android.text.InputType;
import android.view.DisplayCutout;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;
import android.window.OnBackInvokedDispatcher;

import com.orbitrix.mobile.EbitenView;
import com.orbitrix.mobile.Host;
import com.orbitrix.mobile.Mobile;

// MainActivity shows the game full screen. The game itself is Go (package mobile); this class starts it, hides the
// system bars, reports the camera cutout so the layout stays clear of it, and passes the back button on as Esc.
public final class MainActivity extends Activity implements Host {
    private EbitenView view;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Mobile.start(getFilesDir().getAbsolutePath(), this);
        view = new EbitenView(this);
        setContentView(view);

        if (Build.VERSION.SDK_INT >= 28) {
            // Draw under the cutout too: the mat reaches the edges and the game keeps its controls clear of it.
            WindowManager.LayoutParams lp = getWindow().getAttributes();
            lp.layoutInDisplayCutoutMode = WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(lp);
            view.setOnApplyWindowInsetsListener((v, insets) -> {
                DisplayCutout c = insets.getDisplayCutout();
                if (c == null) {
                    Mobile.setSafeArea(0, 0, 0, 0);
                } else {
                    Mobile.setSafeArea(c.getSafeInsetLeft(), c.getSafeInsetTop(), c.getSafeInsetRight(),
                            c.getSafeInsetBottom());
                }
                return v.onApplyWindowInsets(insets);
            });
        }
        if (Build.VERSION.SDK_INT >= 33) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(OnBackInvokedDispatcher.PRIORITY_DEFAULT,
                    Mobile::back);
        }
        hideSystemBars();
    }

    // Before Android 13 back arrives as a key, which the game's view would swallow, so it is caught here first.
    @Override
    public boolean dispatchKeyEvent(KeyEvent e) {
        if (Build.VERSION.SDK_INT < 33 && e.getKeyCode() == KeyEvent.KEYCODE_BACK) {
            if (e.getAction() == KeyEvent.ACTION_UP) {
                Mobile.back();
            }
            return true;
        }
        return super.dispatchKeyEvent(e);
    }

    // The game reads touches once a frame, so a finger that lifts within a frame of landing would never be seen. Such
    // a lift is held back until the touch has lasted MIN_TAP_MILLIS (a frame at 20 FPS).
    private static final long MIN_TAP_MILLIS = 50;
    private Runnable pendingUp;

    @Override
    public boolean dispatchTouchEvent(MotionEvent e) {
        if (pendingUp != null) { // the next touch comes after the held lift
            view.removeCallbacks(pendingUp);
            pendingUp.run();
        }
        long held = e.getEventTime() - e.getDownTime();
        if (e.getActionMasked() == MotionEvent.ACTION_UP && held < MIN_TAP_MILLIS) {
            MotionEvent up = MotionEvent.obtain(e);
            pendingUp = () -> {
                pendingUp = null;
                super.dispatchTouchEvent(up);
                up.recycle();
            };
            view.postDelayed(pendingUp, MIN_TAP_MILLIS - held);
            return true;
        }
        return super.dispatchTouchEvent(e);
    }

    // Immersive: the bars stay hidden, and a swipe from the edge shows them for a moment.
    @SuppressWarnings("deprecation")
    private void hideSystemBars() {
        if (Build.VERSION.SDK_INT >= 30) {
            getWindow().setDecorFitsSystemWindows(false);
            WindowInsetsController c = getWindow().getInsetsController();
            if (c != null) {
                c.hide(WindowInsets.Type.systemBars());
                c.setSystemBarsBehavior(WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
            return;
        }
        getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                | View.SYSTEM_UI_FLAG_FULLSCREEN | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                | View.SYSTEM_UI_FLAG_LAYOUT_STABLE | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION);
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) {
            hideSystemBars(); // they come back after the app switcher and system dialogs
        }
    }

    @Override
    protected void onPause() {
        super.onPause();
        view.suspendGame();
    }

    @Override
    protected void onResume() {
        super.onResume();
        view.resumeGame();
    }

    @Override
    public void leave() {
        runOnUiThread(() -> moveTaskToBack(true));
    }

    // Hosting a room, online or on the LAN, keeps the screen on; the game pauses if the app goes to the background.
    @Override
    public void keepScreenOn(boolean on) {
        runOnUiThread(() -> {
            if (on) {
                getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
            } else {
                getWindow().clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
            }
        });
    }

    @Override
    public boolean copy(String text) {
        ClipboardManager clipboard = (ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        if (clipboard == null) return false;
        try {
            clipboard.setPrimaryClip(ClipData.newPlainText("", text));
            return true;
        } catch (SecurityException e) {
            return false;
        }
    }

    // The game's fields type with the phone's keyboard, through the hidden edit text inside EbitenView. A room code
    // wants capitals and no suggestions; a password must be neither suggested nor learned by the keyboard. The line
    // flag stays, as Ebitengine set it, so Return still reaches the game.
    @Override
    public void keyboard(boolean secret) {
        int type = InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_FLAG_MULTI_LINE
                | InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
                | (secret ? InputType.TYPE_TEXT_VARIATION_PASSWORD : InputType.TYPE_TEXT_FLAG_CAP_CHARACTERS);
        runOnUiThread(() -> {
            for (int i = 0; i < view.getChildCount(); i++) {
                if (view.getChildAt(i) instanceof EditText) {
                    EditText e = (EditText) view.getChildAt(i);
                    if (e.getInputType() != type) {
                        e.setImeOptions(e.getImeOptions() | EditorInfo.IME_FLAG_NO_PERSONALIZED_LEARNING);
                        e.setInputType(type); // restarts the keyboard's input
                    }
                }
            }
        });
    }

    // Wi-Fi drops broadcasts addressed to nobody in particular to save power, LAN hosts' announcements included,
    // unless an app holds the multicast lock. The game holds it only while its Same Wi-Fi page is open.
    private WifiManager.MulticastLock multicast;

    @Override
    public void discover(boolean on) {
        runOnUiThread(() -> {
            if (multicast == null) {
                WifiManager wifi = (WifiManager) getApplicationContext().getSystemService(WIFI_SERVICE);
                multicast = wifi.createMulticastLock("orbitrix");
                multicast.setReferenceCounted(false);
            }
            if (on) {
                multicast.acquire();
            } else {
                multicast.release();
            }
        });
    }

    // The phone's name as its owner set it (Settings, About phone), else its model: what other players see a LAN room
    // listed under.
    @Override
    public String deviceName() {
        String name = Build.VERSION.SDK_INT >= 25
                ? Settings.Global.getString(getContentResolver(), Settings.Global.DEVICE_NAME) : null;
        return name != null && !name.isEmpty() ? name : Build.MODEL;
    }

    // A game on a board with more columns than rows holds the screen on its side (either way up); leaving it hands
    // the orientation back to the phone's own rotation setting.
    @Override
    public void rotate(boolean landscape) {
        runOnUiThread(() -> setRequestedOrientation(landscape
                ? ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
                : ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED));
    }
}
