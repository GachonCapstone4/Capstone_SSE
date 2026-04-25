# SSE Gateway Design Handoff

이 문서는 `SSE/Capstone_SSE`의 현재 Go 구현과 `claude.md` 요구사항을 기준으로 Backend, Frontend, Infra 팀이 맞춰야 할 계약을 정리한다.

## 1. 역할 분리

SSE Gateway는 브라우저와의 장기 SSE 연결만 책임지는 전달 계층이다.

- `SSE/Capstone_SSE`: `/sse/connect` 연결 유지, JWT claim에서 `user_id` 추출, RabbitMQ fanout 메시지를 `user_id`별 열린 연결로 전달
- Backend: 도메인 처리, DB 저장, 작업 상태 갱신, DB commit 이후 `x.sse.fanout` 이벤트 발행
- Frontend: `EventSource`로 `/sse/connect?access_token=...` 구독, 이벤트 수신 후 필요한 화면 데이터를 Backend API로 재조회
- RabbitMQ: 이미 존재하는 `x.sse.fanout` fanout exchange 제공

SSE Gateway가 하지 않는 일:

- 도메인 상태 판단 또는 DB 조회/수정
- RAG, 분류, 템플릿 매칭, 문의 처리 실행
- RabbitMQ exchange 생성
- Backend API 대체 응답 생성
- 사용자별 이벤트 영속화 또는 재전송 보장

## 2. RabbitMQ 계약

SSE Gateway는 시작 시 pod마다 임시 queue를 만들고 `x.sse.fanout`에 bind한다. Exchange는 인프라/서비스 서버가 소유하며 Gateway가 declare하지 않는다.

Queue 생성 기준:

- name: `q.sse.fanout~{uuid}`
- durable: `false`
- auto-delete: `true`
- exclusive: `true`
- arguments: `x-expires = 1800000`

Consume 기준:

- exchange: `x.sse.fanout`
- exchange type: `fanout`
- routing key: 사용하지 않음. 빈 문자열 권장
- ack: 현재 구현은 `auto-ack=true`
- pod fanout: 모든 Gateway pod가 같은 이벤트를 받으며, 각 pod는 자기 메모리에 연결된 `user_id`에게만 전달한다

Backend publish payload:

```json
{
  "user_id": 1,
  "sse_type": "rag-job-updated",
  "data": {
    "job_id": "job-001",
    "status": "PROCESSING"
  }
}
```

필수 필드:

- `user_id`: 수신 사용자 ID. 현재 Gateway는 양의 정수로 해석한다.
- `sse_type`: 브라우저로 보낼 SSE event name.
- `data`: event `data:`에 그대로 실릴 JSON object. Frontend가 파싱 가능한 JSON이어야 한다.

권장 publish 규칙:

- Backend는 DB transaction commit 이후에만 publish한다.
- RabbitMQ 메시지 `content_type`은 `application/json`을 권장한다.
- payload는 UI 갱신 힌트 중심으로 작게 유지하고, 상세 데이터는 Frontend가 Backend API로 재조회한다.
- 민감정보, 원문 이메일 본문, 내부 exception stack trace는 싣지 않는다.

## 3. 이벤트 이름과 payload

현재 Frontend가 실제 구독하는 event name은 아래 4개다.

### `rag-job-updated`

용도: 온보딩 템플릿 생성, 문서 인덱싱, RAG 작업 진행 상태 갱신.

권장 `data`:

```json
{
  "job_id": "job-001",
  "request_id": "req-001",
  "job_type": "templates.index",
  "target_type": "template",
  "target_id": "42",
  "status": "PROCESSING",
  "progress_step": "embedding",
  "progress_message": "회사 자료를 임베딩하고 있습니다.",
  "error_code": null,
  "error_message": null,
  "completed_at": null
}
```

### `classify-complete`

용도: 특정 수신 메일의 AI 분류 완료 알림.

권장 `data`:

```json
{
  "email_id": 123
}
```

### `template-match-updated`

용도: 특정 메일의 추천 템플릿 매칭 결과 갱신 알림.

권장 `data`:

```json
{
  "email_id": 123,
  "recommendation_count": 3
}
```

### `support-ticket-updated`

용도: 사용자 문의 생성, 상태 변경, 관리자 답변 등록 알림.

권장 `data`:

```json
{
  "ticket_id": 77,
  "status": "ANSWERED",
  "admin_reply_present": true,
  "updated_at": "2026-04-24T12:00:00Z"
}
```

`claude.md`에 적힌 `remodeling`, `system-check`는 아직 현재 Frontend 공통 SSE client에 연결되어 있지 않은 후보 이벤트다. Admin 화면에서 사용할 경우 event name과 payload를 별도 합의한 뒤 Frontend 구독 목록에 추가해야 한다.

## 4. Frontend endpoint 계약

현재 Gateway endpoint:

```text
GET /sse/connect?access_token={jwt}
```

현재 구현이 지원하는 인증 입력:

- query: `access_token`
- header: `Authorization: Bearer {jwt}`

브라우저 `EventSource`는 임의 header 설정이 어렵기 때문에 Frontend 기본 계약은 query token 방식이다. Gateway는 JWT의 `sub` 또는 `user_id` claim을 `user_id`로 사용한다.

응답 stream 형식:

```text
event: rag-job-updated
data: {"job_id":"job-001","status":"PROCESSING"}

```

Frontend 동작 기준:

- `VITE_SSE_BASE_URL`이 있으면 `{base}/sse/connect`로 연결한다.
- 값이 없으면 현재 origin의 `/sse/connect`로 연결한다.
- 개발 환경에서는 Vite proxy의 `/sse` target이 Gateway를 바라봐야 한다.
- 운영 환경에서는 Ingress/API gateway가 `/sse/*`를 SSE Gateway service로 route해야 한다.

## 5. Backend 구현 기준

Backend가 구현해야 하는 것:

- `x.sse.fanout`으로 위 envelope를 publish하는 전용 publisher
- 도메인별 이벤트 생성 시점 정의
- DB commit 이후 publish 보장
- publish 실패 로그와 재시도/보상 정책
- 이벤트 payload schema를 DTO 또는 테스트 fixture로 고정

Backend가 구현하지 말아야 하는 것:

- 자체 in-process SSE emitter/controller
- 브라우저 SSE 연결 관리
- Gateway 임시 queue 생성
- `x.sse.fanout` exchange declare
- Gateway가 하는 user connection registry

## 6. 배포와 보안 gap

운영 전 반드시 결정하거나 보강해야 할 항목이다.

- JWT: 현재 Gateway는 JWT 서명을 검증하지 않고 claim만 파싱한다. 운영에서는 서명 검증을 추가하거나, 내부망/Ingress에서 검증된 요청만 Gateway로 들어온다는 정책을 문서화해야 한다.
- Query token: `access_token`이 URL/log에 남을 수 있다. Ingress/access log masking 또는 token 전달 방식 재검토가 필요하다.
- CORS: 현재 구현은 요청 `Origin`을 반사한다. 운영 허용 origin allowlist가 필요하다.
- Heartbeat: 장기 연결 유지를 위한 comment 또는 keepalive event가 없다.
- Graceful shutdown: pod 종료 시 연결 종료와 RabbitMQ consumer 정리를 명시적으로 처리하지 않는다.
- Health check: Kubernetes readiness/liveness endpoint가 없다.
- Proxy buffering: Nginx/Ingress에서 SSE buffering을 꺼야 한다. 앱은 `X-Accel-Buffering: no`를 내려준다.
- Backpressure: user connection channel buffer가 가득 차면 이벤트를 drop한다. 현재 SSE는 best-effort 실시간 알림으로 본다.
- Ack 정책: `auto-ack=true`라 Gateway 장애 중 메시지 유실 가능성이 있다. 상태의 source of truth는 DB이고 Frontend 재조회로 회복한다는 전제가 필요하다.
- 권한: RabbitMQ 계정은 queue declare/bind/consume 권한만 갖고, exchange declare 권한은 없어야 한다.

## 7. 현재 구현 기준 요약

- HTTP server: `SERVER_PORT`, 기본 `8080`
- Endpoint: `/sse/connect`
- RabbitMQ env: `RABBITMQ_HOST`, `RABBITMQ_PORT`, `ADMIN_ID`, `ADMIN_PW`
- Exchange: `x.sse.fanout`
- Queue: pod별 `q.sse.fanout~{uuid}`
- User routing: JWT `sub` 또는 `user_id` claim -> `Hub.clients[userID]`
- Multiple tabs: 같은 user의 여러 connection에 모두 전송
- Missing user or event: `user_id == 0` 또는 빈 `sse_type` 메시지는 drop
