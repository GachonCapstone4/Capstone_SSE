## SSE 게이트웨이

본 서버는 서비스서버의 sse 요청 기능이 늘어남에 따라 별도로 증설하게 된 sse 게이트웨이이다.
클라이언트와 sse 연결을 수립하고 rabbitmq 의 AMQP를 통해 이벤트를 갱신한다.
k8s 환경에서 deployment pod로 동작한다.
RABBITMQ_HOST: "rabbitmq-headless.rabbitmq"
RABBITMQ_PORT: "5672"
를 Config map으로 주입받으며 해당 RabbitMQ의 x.sse.fanout을 구독해야한다.

### 구체적 요구사항
1. 해당 서버는 시작시에 pod 하나당 임시큐를 생성한다. 임시큐의 이름은 q.sse.fanout~(고유식별) 으로 지정한다.
2. api 경로는 /sse/~ 원칙을 지킨다.
3. 채널을 생성해서 클라이언트와 SSE 연결을 맺을 수 있어야한다.
4. 받아오는 queue에는 user_id가 실려있고 해당 user_id 기반으로 emitter 체크를 수행해서 SSE 실시간 동기화를 할수 있어야한다.
5. 서비스 서버에서 Exchange에 어떤 값들을 json 으로 보내줘야 SSE 에 문제가 없는지는 생각해봐야한다.
6. 서비스 서버에서 필요한 sse 요청은 다음과 같다
7. 
- rag-job-updated: 온보딩 템플릿 생성과 인덱싱 같은 RAG 작업 진행 상태를 실시간으로 전달
- classify-complete: 특정 수신 메일의 AI 분류 완료를 알려 수신함 상태와 상세 정보를 빠르게 갱신
- template-match-updated: 특정 메일에 대한 추천 템플릿 매칭 완료를 알려 추천 영역을 갱신
- support-ticket-updated: 사용자 문의 생성 또는 관리자 답변 등록 상태를 알려 문의 화면을 실시간 갱신
- remodeling : 모델 재학습 진행상황을 AI 서버로 부터 받아본다.
- Systecm check: job으로 실행되는 시스템 점검 과정을 실시간으로 터미널 처럼 실시간으로 본다

## 중요 누락된 사항(추가필수)

x.sse.fanout 는 이미 실제 서버에 존재하는 Exchange이다. 절대 본 허브에서 exchange를 생성할 수 있는 권한은 없다.

그리고 서비스 시작시 생성하는 큐에는 위 옵션이 반드시 존재한다.
exclusive = true
auto-delete = true
x-expires: 1800000

