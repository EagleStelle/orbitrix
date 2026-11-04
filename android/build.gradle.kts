// Wraps the game, bound from Go by `ebitenmobile bind` into ../dist/orbitrix.aar, in an installable APK.
// Build it with `go tool mage android`, which binds the AAR first.
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

    // The release key comes from the environment: ORBITRIX_KEYSTORE, a PKCS12 keystore holding the key "orbitrix",
    // and ORBITRIX_KEYSTORE_PASSWORD. Without them the APK is signed with the debug key (CI, testing), which phones
    // refuse to update to or from the release-signed one.
    val keystore = providers.environmentVariable("ORBITRIX_KEYSTORE").orNull
    signingConfigs {
        if (keystore != null) {
            create("release") {
                storeFile = file(keystore)
                storePassword = providers.environmentVariable("ORBITRIX_KEYSTORE_PASSWORD").orNull
                keyAlias = "orbitrix"
                keyPassword = storePassword
            }
        }
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
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
