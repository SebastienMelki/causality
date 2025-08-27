# Contributing to Causality

Thank you for your interest in contributing to Causality! We're excited to have you join our community in building a powerful behavioral analysis system for detecting application modifications.

## 🌟 Ways to Contribute

- **🐛 Bug Reports** - Found something broken? Let us know!
- **💡 Feature Requests** - Have ideas for new functionality?
- **📖 Documentation** - Help improve our docs and examples
- **🔧 Code Contributions** - Fix bugs or implement new features
- **🧪 Testing** - Add test cases or improve test coverage
- **🎨 UI/UX** - Help design and improve the event definition interface
- **📱 Mobile SDKs** - Enhance iOS/Android SDK implementations
- **🌐 WebAssembly** - Improve the WASM SDK and browser integration

## 🚀 Getting Started

### Development Environment Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/yourusername/causality.git
   cd causality
   ```

2. **Install Dependencies**
   ```bash
   # Install Go 1.25 or higher
   # https://golang.org/doc/install
   
   # Install Go dependencies
   go mod tidy
   
   # Install development tools
   make install
   ```

3. **Verify Setup**
   ```bash
   # Run tests
   make test
   
   # Run linting
   make lint
   
   # Generate proto files
   make generate
   ```

### Development Workflow

1. **Create a Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/issue-description
   ```

2. **Make Your Changes**
   - Write clean, well-documented code
   - Follow existing code patterns and style
   - Add tests for new functionality
   - Update documentation as needed

3. **Test Your Changes**
   ```bash
   # Run all tests
   make test
   
   # Run tests with coverage
   make coverage
   
   # Run linting
   make lint
   
   # Test mobile SDK generation
   make mobile
   
   # Test WASM build
   make wasm
   ```

4. **Commit Your Changes**
   ```bash
   # Use conventional commits format
   git commit -m "feat: add new event pattern detection"
   # or
   git commit -m "fix: resolve HTTP request handling issue"
   ```

5. **Push and Create PR**
   ```bash
   git push origin feature/your-feature-name
   ```
   Then create a Pull Request on GitHub

## 📝 Code Style Guidelines

### Go Code

- Follow standard Go conventions and idioms
- Use `gofmt` and `goimports` for formatting
- Write clear, self-documenting code
- Add comments for complex logic
- Keep functions small and focused
- Handle errors explicitly
- Use context for cancellation and timeouts

### Protocol Buffers

- Use clear, descriptive message and field names
- Group related fields together
- Add comments for all services and messages
- Follow [Buf Style Guide](https://docs.buf.build/best-practices/style-guide)
- Version APIs appropriately (v1, v2, etc.)

### Mobile SDK (gomobile)

- Keep platform-specific code minimal
- Use interfaces for platform abstraction
- Handle network failures gracefully
- Implement proper event batching
- Add comprehensive error handling

### WebAssembly

- Minimize binary size
- Use efficient data structures
- Handle browser compatibility issues
- Implement proper error boundaries
- Add performance monitoring

### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code style changes (formatting, etc.)
- `refactor:` Code refactoring
- `test:` Test additions or changes
- `chore:` Maintenance tasks
- `perf:` Performance improvements

Examples:
```
feat: add event collection API to HTTP server
fix: resolve memory leak in analysis engine
docs: update mobile SDK integration guide
test: add unit tests for behavioral pattern matching
perf: optimize event batching for mobile clients
```

## 🧪 Testing Requirements

### Test Coverage

- Maintain minimum 80% code coverage
- Write unit tests for all new functions
- Add integration tests for HTTP API endpoints
- Include edge cases and error scenarios
- Test mobile SDK on both iOS and Android
- Verify WASM builds in multiple browsers

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/analysis

# Run with verbose output
go test -v ./...

# Generate coverage report
make coverage

# Test mobile SDK
make test-mobile

# Test WASM build
make test-wasm
```

### Writing Tests

```go
func TestAnalysisEngine_DetectAnomaly(t *testing.T) {
    // Arrange
    engine := NewAnalysisEngine()
    events := []Event{
        {Type: "login", Timestamp: time.Now()},
        {Type: "suspicious_action", Timestamp: time.Now()},
    }
    
    // Act
    anomaly, err := engine.DetectAnomaly(context.Background(), events)
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, anomaly)
    assert.Equal(t, "suspicious_pattern", anomaly.Type)
}
```

## 🏗️ Architecture Guidelines

### Service Design

- Follow Domain-Driven Design principles
- Keep services loosely coupled
- Use dependency injection
- Implement proper error handling
- Add appropriate logging and metrics

### Event Processing

- Use Protocol Buffers for all event definitions
- Implement efficient serialization/deserialization
- Add event validation and sanitization
- Support event batching and compression
- Handle out-of-order events gracefully

### Analysis Engine

- Implement pluggable detection algorithms
- Support real-time and batch processing
- Add configurable thresholds and rules
- Provide detailed anomaly reports
- Include false positive mitigation

### HTTP Server

- Handle concurrent requests efficiently
- Implement proper connection pooling
- Add rate limiting and throttling
- Support graceful shutdown
- Include API health checks

## 🔍 Code Review Process

### Before Submitting PR

1. **Self-Review Checklist**
   - [ ] Code follows project style guidelines
   - [ ] Tests pass locally
   - [ ] Coverage meets requirements (80%)
   - [ ] Documentation is updated
   - [ ] No debugging code left
   - [ ] Commit messages follow conventions
   - [ ] Mobile SDK builds successfully
   - [ ] WASM compiles without errors

2. **PR Description Template**
   ```markdown
   ## Description
   Brief description of changes
   
   ## Type of Change
   - [ ] Bug fix
   - [ ] New feature
   - [ ] Breaking change
   - [ ] Documentation update
   
   ## Testing
   - [ ] Unit tests pass
   - [ ] Integration tests pass
   - [ ] Manual testing completed
   - [ ] Mobile SDK tested
   - [ ] WASM build tested
   
   ## Checklist
   - [ ] My code follows style guidelines
   - [ ] I have performed self-review
   - [ ] I have added tests
   - [ ] Documentation is updated
   ```

### Review Process

1. All PRs require at least one review
2. Address all feedback constructively
3. Keep PRs focused and small when possible
4. Link related issues in PR description
5. Ensure CI pipeline passes

## 🐛 Reporting Issues

### Bug Reports

When reporting bugs, please include:

1. **Environment Details**
   - OS and version
   - Go version
   - Causality version/commit
   - Mobile platform (if applicable)
   - Browser version (for WASM)

2. **Steps to Reproduce**
   - Clear, numbered steps
   - Minimal reproduction case
   - Expected vs actual behavior

3. **Additional Context**
   - Error messages
   - Log outputs
   - Network traces
   - Screenshots if applicable

### Feature Requests

For feature requests, please describe:

1. **Use Case**
   - What problem does it solve?
   - Who would benefit?
   - Real-world scenario

2. **Proposed Solution**
   - How should it work?
   - API/UI considerations
   - Performance implications

3. **Alternatives Considered**
   - Other approaches
   - Workarounds
   - Trade-offs

## 🎯 Development Priorities

Current focus areas:

1. **Core System**
   - HTTP server scalability
   - Analysis engine accuracy
   - Event processing performance
   - Storage optimization

2. **Mobile SDKs**
   - Battery efficiency
   - Offline support
   - Cross-platform consistency
   - SDK size reduction

3. **WebAssembly**
   - Browser compatibility
   - Performance optimization
   - Bundle size reduction
   - Worker thread support

4. **Analysis Algorithms**
   - Machine learning models
   - Pattern recognition
   - Anomaly detection accuracy
   - Real-time processing

## 📚 Resources

### Documentation
- [README](README.md) - Project overview
- [Architecture](docs/architecture.md) - System design
- [API Docs](docs/api.md) - API specifications
- [Mobile Guide](docs/mobile.md) - Mobile SDK integration
- [WASM Guide](docs/wasm.md) - WebAssembly integration

### External Resources
- [Go Documentation](https://golang.org/doc/)
- [Protocol Buffers](https://protobuf.dev/)
- [gomobile Documentation](https://pkg.go.dev/golang.org/x/mobile)
- [WebAssembly](https://webassembly.org/)

## 🤝 Community

### Communication Channels
- [GitHub Issues](https://github.com/SebastienMelki/causality/issues) - Bug reports and features
- [GitHub Discussions](https://github.com/SebastienMelki/causality/discussions) - General discussion
- [Pull Requests](https://github.com/SebastienMelki/causality/pulls) - Code contributions

### Code of Conduct

We are committed to providing a welcoming and inclusive environment. Please:

- Be respectful and considerate
- Welcome newcomers and help them get started
- Focus on constructive criticism
- Respect differing opinions
- Report inappropriate behavior

## 📜 License

By contributing to Causality, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Causality! Together, we're building the future of behavioral analysis and application security. 🚀🔒