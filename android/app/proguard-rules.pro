# Go/JNI resolves these names and callbacks dynamically. These are the packages
# actually present in ebitenmobile v2.9.11's generated classes.jar; its own AAR
# consumer rules also preserve the binding namespace.
-keep class go.** { *; }
-keep class com.olivierh.populous2.mobile.** { *; }
-keep class com.olivierh.populous2.ebitenmobileview.** { *; }

# Preserve native entry point names if a later binding adds another package.
-keepclasseswithmembernames,includedescriptorclasses class * {
    native <methods>;
}
