// Wraps the game, bound from Go by `ebitenmobile bind` into ../dist/orbitrix.aar, in an installable APK.
// Build it with scripts/build.{sh,ps1} android, which binds the AAR first.
plugins {
    id("com.android.application") version "9.4.1"
}

android {
    namespace = "com.orbitrix"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.orbitrix"
        minSdk = 24
        targetSdk = 37
        versionCode = 1
        versionName = "0.4.0"
    }

    buildTypes {
        release {
            // shortcut: signed with the debug key so friends can install it directly; use a real keystore before any
            // store release.
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    // Compressed native code: a smaller APK to pass around (the game is almost all native code).
    packaging {
        jniLibs {
            useLegacyPackaging = true
        }
    }
}

dependencies {
    implementation(files("../dist/orbitrix.aar"))
}
