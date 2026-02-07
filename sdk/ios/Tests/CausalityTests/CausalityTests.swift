import XCTest
@testable import CausalitySwift

final class CausalityTests: XCTestCase {

    func testConfigEncoding() throws {
        let config = Config(
            apiKey: "test-key",
            endpoint: "https://api.example.com",
            appId: "test-app",
            batchSize: 50,
            debugMode: true
        )

        let encoder = JSONEncoder()
        let data = try encoder.encode(config)
        let json = String(data: data, encoding: .utf8)!

        XCTAssertTrue(json.contains("\"api_key\":\"test-key\""))
        XCTAssertTrue(json.contains("\"app_id\":\"test-app\""))
        XCTAssertTrue(json.contains("\"batch_size\":50"))
    }

    func testEventBuilder() {
        let event = EventBuilder(type: "purchase")
            .property("product_id", "abc123")
            .property("price", 29.99)
            .property("quantity", 2)
            .build()

        XCTAssertEqual(event.type, "purchase")
        XCTAssertNotNil(event.properties)
        XCTAssertEqual(event.properties?["product_id"]?.value as? String, "abc123")
        XCTAssertEqual(event.properties?["price"]?.value as? Double, 29.99)
        XCTAssertEqual(event.properties?["quantity"]?.value as? Int, 2)
    }

    func testEventEncoding() throws {
        let event = Event(
            type: "test_event",
            properties: [
                "string_prop": AnyCodable("value"),
                "int_prop": AnyCodable(42),
                "bool_prop": AnyCodable(true)
            ]
        )

        let encoder = JSONEncoder()
        let data = try encoder.encode(event)
        let json = String(data: data, encoding: .utf8)!

        XCTAssertTrue(json.contains("\"type\":\"test_event\""))
    }

    func testAnyCodableRoundtrip() throws {
        let original = AnyCodable("test string")

        let encoder = JSONEncoder()
        let data = try encoder.encode(original)

        let decoder = JSONDecoder()
        let decoded = try decoder.decode(AnyCodable.self, from: data)

        XCTAssertEqual(decoded.value as? String, "test string")
    }
}
