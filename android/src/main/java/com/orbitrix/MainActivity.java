package com.orbitrix;

import android.app.Activity;
import android.os.Build;
import android.os.Bundle;
import android.view.DisplayCutout;
import android.view.KeyEvent;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
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
}
