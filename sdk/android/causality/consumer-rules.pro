# Keep Go mobile generated code
-keep class io.causality.mobile.** { *; }
-keep class mobile.** { *; }

# Keep kotlinx.serialization
-keepattributes *Annotation*, InnerClasses
-dontnote kotlinx.serialization.AnnotationsKt
-keepclassmembers class kotlinx.serialization.json.** {
    *** Companion;
}
-keepclasseswithmembers class kotlinx.serialization.json.** {
    kotlinx.serialization.KSerializer serializer(...);
}
-keep,includedescriptorclasses class io.causality.**$$serializer { *; }
-keepclassmembers class io.causality.** {
    *** Companion;
}
-keepclasseswithmembers class io.causality.** {
    kotlinx.serialization.KSerializer serializer(...);
}
