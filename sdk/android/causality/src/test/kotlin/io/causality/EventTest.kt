package io.causality

import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.junit.Assert.*
import org.junit.Test

class EventTest {
    private val json = Json { encodeDefaults = false }

    @Test
    fun `event DSL creates valid event`() {
        val event = event("purchase") {
            property("product_id", "abc123")
            property("price", 29.99)
            property("quantity", 2)
            property("in_stock", true)
        }

        assertEquals("purchase", event.type)
        assertNotNull(event.properties)
    }

    @Test
    fun `event serializes correctly`() {
        val event = event("test_event") {
            property("string_prop", "value")
            property("int_prop", 42)
        }

        val jsonString = json.encodeToString(event)

        assertTrue(jsonString.contains("\"type\":\"test_event\""))
        assertTrue(jsonString.contains("\"string_prop\":\"value\""))
        assertTrue(jsonString.contains("\"int_prop\":42"))
    }

    @Test
    fun `event without properties serializes null`() {
        val event = event("simple_event")

        val jsonString = json.encodeToString(event)

        assertTrue(jsonString.contains("\"type\":\"simple_event\""))
        // properties should be null/absent
    }

    @Test
    fun `event builder supports all property types`() {
        val event = event("typed_event") {
            property("string", "value")
            property("int", 42)
            property("long", 9999999999L)
            property("double", 3.14159)
            property("bool", true)
        }

        assertEquals("typed_event", event.type)
        assertEquals(5, event.properties?.size)
    }
}
