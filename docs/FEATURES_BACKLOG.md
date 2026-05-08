# Stock Tracker — Ideas para el Backlog

> Generado: 2026-03-25
> Última actualización: 2026-05-08
> Status: Backlog — priorizado por impacto y esfuerzo

---

## ✅ Lo que YA está implementado (al 2026-05-08)

| # | Idea | Estado | PR | Fecha |
|---|------|--------|-----|-------|
| 9 | Dashboard Web | ✅ **COMPLETADO** | #58 | 2026-03-30 |
| 8 | Asignación por Sector | ✅ **COMPLETADO** | #69 | 2026-05-08 |
| 2 | Ventas Parciales | ✅ **COMPLETADO** | #72 | 2026-05-08 |

**Detalles:**
- **Dashboard Web (#58):** Endpoint `/dashboard` con UI ligera, sparklines, tabla de posiciones y breakdown de asignación. Auto-hosted, cero dependencias frontend.
- **Asignación por Sector (#69):** Breakdown por tipo (ETF/Stock) y sector, con warnings automáticos cuando alguna concentración excede 40%.
- **Ventas Parciales (#72):** Endpoint `POST /api/v1/positions/:id/sell` para vender parcial o totalmente posiciones. Calcula P/L proporcional, actualiza invested amount restante, y registra SaleTransaction para auditoría.

---

## Resumen de Priorización Rápida

| # | Idea | Impacto | Esfuerzo | Estado | Próximo paso |
|---|------|---------|----------|--------|---------------|
| ~~9~~ | ~~Dashboard Web~~ | ~~🔴 Alto~~ | ~~🟡 Medio~~ | ✅ **HECHO** (PR #58) | — |
| ~~8~~ | ~~Asignación por Sector~~ | ~~🟢 Bajo~~ | ~~🟢 Bajo~~ | ✅ **HECHO** (PR #69) | — |
| ~~2~~ | ~~Ventas Parciales~~ | ~~🔴 Alto~~ | ~~🟡 Medio~~ | ✅ **HECHO** (PR #72) | — |
| 1 | Multi-Portfolio | 🔴 Alto | 🟡 Medio | ⬜ Pendiente | 🚨 Empezar aquí |
| 3 | Historial de Transacciones | 🟡 Medio | 🟡 Medio | ⬜ Pendiente | 🔶 Segundo lote |
| 4 | Multi-Currency | 🟡 Medio | 🟡 Medio | ⬜ Pendiente | 🔶 Segundo lote |
| 11 | Dividendos | 🟡 Medio | 🟡 Medio | ⬜ Pendiente | 🔶 Segundo lote |
| 7 | Sistema de Alertas | 🟡 Medio | 🟡 Medio | ⬜ Pendiente | 🔷 Tercer lote |
| 6 | Analytics (XIRR) | 🟡 Medio | 🔴 Alto | ⬜ Pendiente | 🔷 Tercer lote |
| 12 | Rebalancing Advisor | 🟡 Medio | 🟡 Medio | ⬜ Pendiente | 🔷 Tercer lote |
| 5 | WebSocket | 🟡 Medio | 🔴 Alto | ⬜ Pendiente | 🔷 Tercer lote |
| 10 | Kubernetes/Helm | 🟢 Bajo | 🟢 Bajo | ⬜ Pendiente | 🔷 Tercer lote |

---

## 🚨 Lote 1 — Lo que hace el tracker completo

### 1. 🚀 Multi-Portfolio System

**Situación actual:** Solo existe un portafolio "default". Todo se mezcla.

**Qué añade:** Sistema de múltiples portafolios con nombres, tipos (acciones, ETF, crypto, jubilación), y metadata. Un usuario puede tener portafolios temáticos.

**Valor:** Organización real, diversificación mental clara.

**Dificultad:** Media — ya existe `FindAll` en el repositorio, solo falta la capa de negocio y endpoints.

**Detalles técnicos:**
- Nuevo modelo `Portfolio` ya existe — solo necesita un campo `type` o `tags`
- Nuevos endpoints: `POST /api/v1/portfolios`, `GET /api/v1/portfolios`, `GET /api/v1/portfolios/:id`, `DELETE /api/v1/portfolios/:id`
- El `PortfolioService` actual se renombra a `DefaultPortfolioService` o se hace `MultiPortfolioService`
- Migración: añadir columna `type VARCHAR(50)` a `portfolios`

---

### ~~2. 💸 Ventas Parciales (Partial Sell)~~ ✅ **COMPLETADO 2026-05-08**

**Estado:** Implementado en PR #72.

**Qué se implementó:**
- ✅ Nuevo endpoint: `POST /api/v1/positions/:id/sell`
- ✅ Modelo `SaleTransaction` para registro de auditoría
- ✅ Método `Portfolio.SellPartial()` con validaciones
- ✅ Cálculo proporcional de invested amount y P/L
- ✅ Soporte para venta parcial y venta total (remove position)
- ✅ 8 tests unitarios cubriendo edge cases

**Request:**
```json
{ "quantity": "5", "price": "120.50" }
```

**Response:**
```json
{
  "sale": {
    "quantity_sold": "5",
    "sale_price": "120.50",
    "total_proceeds": "602.50",
    "invested_sold": "500",
    "profit_loss": "102.50",
    "remaining_qty": "5",
    "remaining_invest": "500",
    "is_full_sale": false
  },
  "position": { ... },
  "is_full_sale": false
}
```

---

## 🔶 Lote 2 — Hacerlo accesible y completo

### ~~9. 📉 Interfaz Dashboard Web~~ ✅ **COMPLETADO 2026-03-30**

**Estado:** Implementado en PR #58. Dashboard disponible en `/dashboard` con:
- Summary cards (total value, P/L, positions count)
- Allocation breakdown (by type: ETF vs Stock)
- Positions table sortable
- Sparklines de 7 días para cada posición
- Cero dependencias frontend (vanilla JS + SVG)

**Pendiente:** El breakdown es solo por tipo (etf/stock), falta por sector específico (ver #8).

---

### 3. 📜 Historial de Transacciones (Transaction Log)

**Situación actual:** Solo existe el estado actual. No hay forma de saber cuándo compraste o vendiste.

**Qué añade:** Tabla `transactions` con tipo (`buy`, `sell`, `dividend`, `split`), timestamp, cantidad, precio, y fees. El portfolio summary incluye un array de transacciones recientes.

**Valor:** Auditoría completa de decisiones de inversión. Base para reportes fiscales.

**Detalles técnicos:**
- Nueva tabla: `transactions(id, portfolio_id, position_id, type, quantity, price, fees, currency, timestamp)`
- `AddPosition` crea un registro `buy` automáticamente
- `SellPosition` (idea 2) crea un registro `sell`
- Nuevo endpoint: `GET /api/v1/portfolio/transactions?limit=50&offset=0`
- Los tests E2E ya tienen este concepto — se puede seguir el mismo patrón

---

### 4. 💱 Conversión de Monedas (Multi-Currency)

**Situación actual:** Las posiciones en USD y EUR no se pueden agregar — no hay conversión.

**Qué añade:** Provider de forex (gratuito con Frankfurter o exchangerate.host). Todos los valores del portfolio summary se muestran en una moneda base configurable. `TotalValue` convierte cada posición antes de sumar.

**Valor:** Portafolios reales tienen posiciones en múltiples divisas. Sin esto, el "total" es engañoso.

**Detalles técnicos:**
- Nuevo `ForexProvider` interface: `GetRate(ctx, from, to string) (Decimal, error)`
- Implementación trivial con Frankfurter API gratuita: `https://api.frankfurter.app/latest?from=USD&to=EUR`
- Cache en memoria con TTL 1 hora (los rates no cambian tan seguido)
- Campo en `portfolio`: `base_currency string` (default "EUR")
- Modificar `TotalValue()`, `TotalProfitLoss()` para convertir antes de operar
- Nuevo endpoint: `GET /api/v1/portfolio?base_currency=USD`

---

### 9. 📉 Interfaz Dashboard Web

**Situación actual:** Solo API REST. Para ver el portafolio usas curl.

**Qué añade:** Frontend ligero en Go HTML templates o un React companion. Gráficos de línea para P/L en el tiempo, donut chart para asignación, tabla de posiciones sortable. Self-hosted dashboard.

**Valor:** Accesibilidad. Un dashboard visual convierte esta tool de "interesante" a "usable a diario".

**Detalles técnicos:**
- Opción rápida: Gin + Go templates + Chart.js ( vanilla JS)
- Opción robusta: Next.js o Vite+React en `web/` folder separado
- Endpoints necesarios ya existen — solo agregar `GET /api/v1/portfolio/history` (idea 3)
- Lo mínimo viable: una página HTML que haga fetch a `/api/v1/portfolio` y renderice con JavaScript

---

### 11. 🏦 Seguimiento de Dividendos

**Situación actual:** No se trackean dividendos.

**Qué añade:** Nuevo modelo `Dividend` con ISIN, fecha ex-dividend, fecha pago, cantidad por acción, y moneda. Endpoint `GET /api/v1/positions/{id}/dividends`. Resumen anual de dividendos recibidos. **Yield on cost** por posición.

**Valor:** Los dividendos son una parte enorme del retorno total a largo plazo. Sin tracking son invisibles.

**Detalles técnicos:**
- Nueva tabla: `dividends(id, position_id, isin, ex_date, pay_date, amount_per_share, currency)`
- YFinance API incluye datos de dividendos: `yfinance.Ticker.info['dividendRate']`, `yfinance.Ticker.dividends`
- Nuevo endpoint: `GET /api/v1/positions/{id}/dividends`
- En `GetPortfolio`: añadir `total_dividends_ytd`, `yield_on_cost` por posición
- Campo nuevo en `Position`: `DividendYield` calculado como `(annual_dividend / current_price) * 100`

---

## 🔷 Lote 3 — Optimización y proactividad

### 5. 📡 WebSocket para Precios en Tiempo Real

**Situación actual:** `PriceUpdater` hace polling cada 60s (pull). Eso son 1.440 requests/día por usuario como máximo.

**Qué añade:** Canal WebSocket (`/ws/prices`) que recibe actualizaciones push cuando los precios cambian.

**Valor:** Eliminación de delay de hasta 60s en precios. Experiencia tipo Bloomberg.

**Detalles técnicos:**
- Gin no soporta nativamente WebSocket — usar `github.com/gorilla/websocket`
- Patron Hub: mapa de clientes, broadcast channel
- `PriceUpdater` actual envía a un canal interno, el hub hace el broadcast
- Los providers gratuitos (YFinance) no tienen WebSocket push — el polling fino (cada 15s?) sigue siendo necesario
- Endpoint: `GET /ws/prices` — upgrade a WebSocket, envía JSON con `{symbol, price, time}`

---

### 6. 📊 Analytics: XIRR y Rendimiento Ajustado por Riesgo

**Situación actual:** Solo P/L simple (`current_value - invested`). No considera timing.

**Qué añade:** Cálculo de **XIRR** (internal rate of return con fechas reales de cada transacción) usando Newton-Raphson en Go. Además: Sharpe ratio simplificado, max drawdown, y volatility.

**Valor:** Saber realmente si tu portafolio rindió bien o si fue suerte de timing.

**Detalles técnicos:**
- XIRR requiere la idea 3 (Transaction Log) para tener fechas reales
- Newton-Raphson: función objetivo = NPV = Σ(CF_t / (1+r)^t) — buscar r tal que NPV = 0
- Convergencia típica en 10-20 iteraciones con guess inicial de 0.1
- Sharpe ratio: `(return - risk_free_rate) / volatility` — risk-free puede ser hardcodeado o de API
- Max drawdown: mayor caída desde un peak hasta un trough en el historial de valores

---

### 7. 🔔 Sistema de Alertas (Price Alerts)

**Situación actual:** No hay notificaciones. Solo puedes entrar a preguntar.

**Qué añade:** Endpoints `POST /api/v1/alerts` para crear alertas (precio > o < threshold, % change diario, P/L% position). Alertas se evaluan en cada `RefreshPrices`. Delivery via Telegram/Email/Push.

**Valor:** Interactividad real. El usuario no tiene que estar mirando constantemente.

**Detalles técnicos:**
- Nueva tabla: `alerts(id, position_id, type, threshold, triggered, created_at)`
- Tipos: `price_above`, `price_below`, `pl_percent_above`, `pl_percent_below`, `daily_change_percent`
- Evaluación en `RefreshPrices()` loop — cheap, ya se itera sobre todas las posiciones
- Delivery: interface `AlertNotifier` con implementaciones para Telegram, Email, Webhook
- Endpoints: `GET /api/v1/alerts`, `POST /api/v1/alerts`, `DELETE /api/v1/alerts/:id`

---

### ~~8. 🗺️ Asignación por Sector/Asset Class~~ ✅ **COMPLETADO (2026-05-08)**

**Estado:** Completado en PR #69. El dashboard ahora muestra:
- ✅ Allocation breakdown por tipo de activo (ETF vs Stock)
- ✅ Allocation breakdown por sector (Technology, Healthcare, Finance, etc.)
- ✅ Warnings automáticos cuando sector o tipo excede 40% de concentración
- ✅ Porcentajes y valores absolutos

**Implementación:**
- Backend: `DashboardSnapshot.Warnings` calcula warnings automáticamente
- Frontend: Tarjeta roja con warnings aparece sobre las allocation cards
- Tests: Unit tests para validación de lógica de warnings

**Qué añade:** En el `GET /portfolio` response, un breakdown por tipo de activo (`etf`, `stock`) y por sector. Incluye % del portafolio, y alerta si algún sector > 40%.

**Valor:** Diversificación real visible.

**Detalles técnicos:**
- Añadir `sector` y `region` a `Instrument` — enriquecer desde metadata del provider o hardcodear para ETFs comunes
- `GetPortfolio` response incluye:
```json
{
  "allocation": {
    "by_type": {
      "etf": { "value": 8000, "percent": 72.7 },
      "stock": { "value": 3000, "percent": 27.3 }
    },
    "by_sector": {
      "equity_world": { "value": 6000, "percent": 54.5 },
      "equity_usa": { "value": 3000, "percent": 27.3 },
      "equity_em": { "value": 2000, "percent": 18.2 }
    },
    "warnings": ["sector equity_usa exceeds 40%"]
  }
}
```

---

### 10. 🐳 Kubernetes + Helm Chart

**Situación actual:** Solo `docker compose` para desarrollo local.

**Qué añade:** Helm chart con `values.yaml` configurable, liveness/readiness probes, `HorizontalPodAutoscaler` basado en CPU.

**Detalles técnicos:**
- Estructura estándar Helm: `Chart.yaml`, `values.yaml`, `templates/`
- Templates: `deployment.yaml`, `service.yaml`, `hpa.yaml`, `ingress.yaml`
- El `docker-compose.yml` actual es la especificación de servicios — convertir a Helm values
- Añadir resource limits y requests para HPA
- Compatible con k3s, GKE, EKS

---

### 12. 🧠 Intelligent Rebalancing Advisor

**Situación actual:** El usuario decide solo qué comprar/vender.

**Qué añade:** Endpoint `GET /api/v1/portfolio/rebalance` que compara la asignación actual vs target (ej: 60% acciones, 40% bonos), sugiere qué vender y qué comprar para volver al target.

**Detalles técnicos:**
- Input: targets por tipo de activo en la request o en el portfolio metadata
- Output: lista de `[{action: "sell", isin: "XXX", amount: 500}, {action: "buy", isin: "YYY", amount: 500}]`
- Lógica: 1) calcular desviación de cada posición vs target, 2) las que exceeden el target se venden, 3) las que están por debajo se compran
- Threshold de desviación configurable (default 5%)
- Las ventas se hacen primero para generar cash antes de las compras (evitar problema de timing)

---

## Notas de Implementación

- **Tests:** Los E2E con Testcontainers y containers compartidos ya están listos — sirven como base para todos los nuevos features.
- **Decimal precision:** El proyecto usa `cockroachdb/apd` con precision 20 — suficiente para todos los cálculos financieros incluyendo XIRR.
- **Provider interface:** La arquitectura de market data providers es extensible — cualquier nueva feature de datos puede usar el mismo patrón de `MDataProvider` / `BatchProvider`.

## Orden Sugerido Actualizado (al 2026-05-08)

**Completado:** ~~9~~ (Dashboard Web), ~~8~~ (Asignación por Sector), ~~2~~ (Ventas Parciales)

**Próximo:** 1 → 3 → 4 → 11 → 7 → 12 → 6 → 5 → 10

1. **Multi-Portfolio (#1)** — Organización básica que todo usuario necesita
2. **Historial de Transacciones (#3)** — Necesario para auditoría y base para XIRR
3. **Multi-Currency (#4)** — Portafolios reales tienen múltiples divisas
4. **Multi-Currency (#4)** — Portafolios reales tienen múltiples divisas
5. **Dividendos (#11)** — Tracking de retorno total
6. **Alertas (#7)** — Interactividad proactiva
7. **Rebalancing Advisor (#12)** — Optimización de portafolio
8. **XIRR (#6)** — Analytics avanzado (requiere #3)
9. **WebSocket (#5)** — Real-time (optimización)
10. **K8s/Helm (#10)** — Deploy production (infraestructura)
