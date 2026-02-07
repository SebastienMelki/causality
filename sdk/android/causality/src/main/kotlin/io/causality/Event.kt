package io.causality

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

@Serializable
data class Event(
    val type: String,
    val properties: JsonObject? = null,
    val timestamp: String? = null
)

class EventBuilder(private val type: String) {
    private val properties = mutableMapOf<String, JsonElement>()

    fun property(key: String, value: String): EventBuilder {
        properties[key] = JsonPrimitive(value)
        return this
    }

    fun property(key: String, value: Int): EventBuilder {
        properties[key] = JsonPrimitive(value)
        return this
    }

    fun property(key: String, value: Long): EventBuilder {
        properties[key] = JsonPrimitive(value)
        return this
    }

    fun property(key: String, value: Double): EventBuilder {
        properties[key] = JsonPrimitive(value)
        return this
    }

    fun property(key: String, value: Boolean): EventBuilder {
        properties[key] = JsonPrimitive(value)
        return this
    }

    fun build(): Event {
        return Event(
            type = type,
            properties = if (properties.isEmpty()) null else JsonObject(properties)
        )
    }
}

inline fun event(type: String, block: EventBuilder.() -> Unit = {}): Event {
    return EventBuilder(type).apply(block).build()
}
