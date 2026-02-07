package io.causality

import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import org.junit.Assert.*
import org.junit.Test

class ConfigTest {
    private val json = Json { encodeDefaults = false }

    @Test
    fun `config DSL creates valid config`() {
        val config = config {
            apiKey = "test-key"
            endpoint = "https://api.example.com"
            appId = "test-app"
            batchSize = 50
            debugMode = true
        }

        assertEquals("test-key", config.apiKey)
        assertEquals("https://api.example.com", config.endpoint)
        assertEquals("test-app", config.appId)
        assertEquals(50, config.batchSize)
        assertEquals(true, config.debugMode)
    }

    @Test
    fun `config serializes with snake_case`() {
        val config = Config(
            apiKey = "test-key",
            endpoint = "https://api.example.com",
            appId = "test-app",
            batchSize = 50
        )

        val jsonString = json.encodeToString(config)

        assertTrue(jsonString.contains("\"api_key\":\"test-key\""))
        assertTrue(jsonString.contains("\"app_id\":\"test-app\""))
        assertTrue(jsonString.contains("\"batch_size\":50"))
    }

    @Test(expected = IllegalArgumentException::class)
    fun `config builder requires apiKey`() {
        config {
            endpoint = "https://api.example.com"
            appId = "test-app"
        }
    }

    @Test(expected = IllegalArgumentException::class)
    fun `config builder requires endpoint`() {
        config {
            apiKey = "test-key"
            appId = "test-app"
        }
    }

    @Test(expected = IllegalArgumentException::class)
    fun `config builder requires appId`() {
        config {
            apiKey = "test-key"
            endpoint = "https://api.example.com"
        }
    }
}
