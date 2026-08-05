# Enterprise Architecture - dbBack v3.0+

## 📋 Visão Geral

Sistema de caching inteligente, resiliência, rate limiting avançado e WebAuthn credential management profissional.

---

## 🏗️ Camadas da Arquitetura

### 1. **Caching Layer** (Multi-Strategy)
```
┌─────────────────────────────────────────┐
│   L1: In-Memory Cache (Hot Data)        │ ← Cache-Aside + Write-Through
│   - TTL: 5 minutos                      │
│   - Eviction: LRU                       │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│   L2: Disk Cache (Warm Data)            │ ← Write-Through + Stampede Guard
│   - TTL: 1-7 horas                      │
│   - Size: 500MB max                     │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│   L3: Google Sheets (Source of Truth)   │ ← Circuit Breaker + Fallback
│   - Rate Limited: 300 req/min           │
│   - Retry: Exponential Backoff          │
└─────────────────────────────────────────┘
```

### 2. **Rate Limiting** (Multi-Level)
```
Token Bucket Algorithm:
- Per-user limit: 100 req/min
- Per-IP limit: 500 req/min
- Per-endpoint: 1000 req/min
- Burst capacity: 2x

Leaky Bucket (fallback):
- Steady flow rate
- Queue max: 50 requests
```

### 3. **Resiliência** (Fail-Safe)
```
Circuit Breaker:
  CLOSED → detects failure → OPEN → timeout → HALF_OPEN → success/failure → CLOSED/OPEN

Retry Strategy:
  Attempt 1 (immediate)
  ↓ (1s delay)
  Attempt 2
  ↓ (2s delay)
  Attempt 3
  ↓ (4s delay)
  Attempt 4
  ↓ (8s delay)
  Attempt 5 (fail)

Fallback: Use disk cache + stale data
```

### 4. **WebAuthn Credential Management**
```
First Login (New Device):
  ✓ Authenticate with WebAuthn
  → Detect provider (Apple Passkey / Google)
  → Save credential with ID + provider
  → Set isSaved flag

Second Login (Same Device):
  ✓ Check isSaved
  → If YES: Don't ask again
  → If NO: Ask "Save this credential?"

Different Credential (Same Device):
  ✓ Detect credential change
  → Ask again "Save this new credential?"
```

### 5. **CAP Theorem Trade-offs**
```
Choose: Consistency + Availability (sacrifice Partition Tolerance)

Implications:
- Strong consistency for grades (financial impact)
- Eventual consistency for feedback (non-critical)
- No split-brain scenarios (central Google Sheets)
```

### 6. **Two-Phase Commit** (Transactional Safety)
```
Phase 1 (Prepare):
  ✓ Validate data integrity
  ✓ Check resources available
  ✓ Lock resources
  ✓ Vote: commit or rollback

Phase 2 (Commit):
  ✓ Execute all changes atomically
  ✓ Release locks
  ✓ Log transaction
```

### 7. **Observability Stack**
```
┌──────────────┐
│   Logging    │ ← Structured logs (JSON)
├──────────────┤
│   Profiling  │ ← CPU, Memory, Goroutines
├──────────────┤
│   Metrics    │ ← Response time, Cache hit rate
├──────────────┤
│   Tracing    │ ← Request flow across services
└──────────────┘
```

---

## 🔧 Implementação

### Pattern: Cache-Aside
```go
// 1. Try L1 (Memory)
if cached := l1Cache.Get(key); cached != nil {
  return cached, nil
}

// 2. Try L2 (Disk)
if cached := l2Cache.Get(key); cached != nil {
  go l1Cache.Set(key, cached) // Async backfill
  return cached, nil
}

// 3. Try source (Google Sheets)
data := fetchFromSheets(key)
go l1Cache.Set(key, data)
go l2Cache.Set(key, data)
return data, nil
```

### Pattern: Write-Through
```go
// Write to both layers atomically
tx := beginTransaction()
defer tx.Rollback()

tx.Write(l1Cache, key, value)
tx.Write(l2Cache, key, value)
tx.Write(googleSheets, key, value)

tx.Commit() // All or nothing
```

### Pattern: Stampede Guard
```go
// Prevent multiple concurrent fetches for same key
mutex := cache.locks[key]
if mutex.TryLock() {
  defer mutex.Unlock()
  
  // Double-check after lock
  if cached := cache.Get(key); cached != nil {
    return cached, nil
  }
  
  // Safe to fetch
  data := fetch()
  cache.Set(key, data)
  return data, nil
}

// Wait for other goroutine
<-mutex.Done()
return cache.Get(key), nil
```

### Pattern: Circuit Breaker
```go
type CircuitBreaker struct {
  state       State // CLOSED, OPEN, HALF_OPEN
  failCount   int
  lastFailAt  time.Time
  halfOpenAt  time.Time
  threshold   int // failures to open
  timeout     time.Duration
}

func (cb *CircuitBreaker) Call(fn func() error) error {
  if cb.state == OPEN {
    if time.Since(cb.lastFailAt) > cb.timeout {
      cb.state = HALF_OPEN
      cb.halfOpenAt = time.Now()
    } else {
      return ErrCircuitOpen
    }
  }
  
  err := fn()
  
  if err != nil {
    cb.failCount++
    cb.lastFailAt = time.Now()
    if cb.failCount >= cb.threshold {
      cb.state = OPEN
    }
    return err
  }
  
  // Success
  cb.failCount = 0
  cb.state = CLOSED
  return nil
}
```

### Pattern: Token Bucket
```go
type TokenBucket struct {
  capacity    float64
  refillRate  float64 // tokens/second
  tokens      float64
  lastRefill  time.Time
}

func (tb *TokenBucket) Allow(cost float64) bool {
  now := time.Now()
  elapsed := now.Sub(tb.lastRefill).Seconds()
  
  tb.tokens = math.Min(tb.capacity, tb.tokens + elapsed * tb.refillRate)
  tb.lastRefill = now
  
  if tb.tokens >= cost {
    tb.tokens -= cost
    return true
  }
  
  return false
}
```

### Pattern: Exponential Backoff
```go
func retryWithBackoff(fn func() error, maxAttempts int) error {
  for attempt := 1; attempt <= maxAttempts; attempt++ {
    err := fn()
    if err == nil {
      return nil
    }
    
    if attempt < maxAttempts {
      backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
      time.Sleep(backoff)
    }
  }
  
  return ErrMaxRetriesExceeded
}
```

### Pattern: Idempotency
```go
type IdempotencyKey struct {
  ClientId  string
  RequestId string
  Timestamp int64
}

func (s *Service) GradeRequest(idempotencyKey string) (Grade, error) {
  // Check if already processed
  if cached := s.idempotencyCache.Get(idempotencyKey); cached != nil {
    return cached.(Grade), nil
  }
  
  // Process request
  grade := s.calculateGrade()
  
  // Cache result for 24h
  s.idempotencyCache.Set(idempotencyKey, grade)
  
  return grade, nil
}
```

---

## 🔐 WebAuthn Credential Management (Smart)

### Estrutura de Dados
```go
type Credential struct {
  ID              string    // public key credential ID
  Provider        string    // "apple", "google", "webauthn"
  PublicKey       []byte    // encoded public key
  Counter         uint32    // attack prevention
  CreatedAt       time.Time
  LastUsedAt      time.Time
  IsDefault       bool
  SavedByUser     bool      // user consent flag
}

type CredentialSession struct {
  UserId         string
  Credentials    []Credential
  CurrentId      string    // credential used in this session
  WasKnown       bool      // was this credential known?
}
```

### Lógica de Detecção
```go
func detectProvider(clientResponse) string {
  // Analyze client response to determine provider
  // Apple: attestation format = "apple"
  // Google: clientData contains google.com origin
  // Generic: standard webauthn
  
  if response.Attestation == "apple" {
    return "apple"
  }
  if strings.Contains(response.ClientData, "google") {
    return "google"
  }
  return "webauthn"
}
```

### State Machine
```
FIRST_LOGIN:
  ✓ Register credential
  → Ask: "Save this [Apple/Google] credential?"
  → store.SavedByUser = true/false
  
SECOND_LOGIN (known credential):
  ✓ Authenticate
  → Check: store.SavedByUser
  → If true: Don't ask
  → If false: Ask "Save for next time?"
  
DIFFERENT_CREDENTIAL (same device):
  ✓ Detect: currentId != session.currentId
  → Ask: "Save this new [Apple/Google] credential?"
  → store.SavedByUser = true/false
```

---

## 📊 Trade-offs Explícitos

| Decision | Benefit | Cost |
|----------|---------|------|
| **L1 + L2 Cache** | Fast reads | Memory usage (+100MB) |
| **Token Bucket** | Fair rate limiting | Computational overhead |
| **Circuit Breaker** | Fail fast | Temporary unavailability |
| **Exponential Backoff** | Reduced thundering herd | Longer retry time |
| **Consistency over Partition** | No stale grades | Requires connectivity |
| **Structured Logging** | Easy debugging | Log storage cost |
| **Per-User Rate Limit** | Fair for all users | More state to manage |

---

## 🧪 Testing Strategy

1. **Unit Tests**: Each pattern in isolation
2. **Integration Tests**: Cache layers together
3. **Load Tests**: Rate limiter under stress
4. **Chaos Tests**: Circuit breaker behavior
5. **Scenario Tests**: WebAuthn flows

---

## 📈 Metrics to Monitor

```
Cache:
  - Hit rate (L1, L2)
  - Eviction rate
  - Fill time

Rate Limiting:
  - Requests rejected (per bucket)
  - Burst detections
  - Recovery time

Circuit Breaker:
  - State transitions
  - Half-open attempts
  - Recovery time

Credentials:
  - Provider distribution
  - Save consent rate
  - Multi-credential usage

General:
  - P95, P99 latencies
  - Error rate by type
  - Goroutine count
  - Memory allocation rate
```

---

## 🚀 Implementation Phases

**Phase 1**: Cache Stampede Guard + Write-Through
**Phase 2**: Circuit Breaker + Retries  
**Phase 3**: Token Bucket v2 + Leaky Bucket
**Phase 4**: WebAuthn Credential Management
**Phase 5**: Observability & Profiling
**Phase 6**: Load Testing & Optimization

---

Generated with Enterprise Architecture Principles
