# AI Billing / Usage Stats — Android Integration Guide

**Сервер:** v1.3.0.4+ | **Дата:** 2026-06-20

Интеграция экрана статистики использования AI (токены, запросы) в Android клиенте.

---

## API

### GetAIUsageStats

```protobuf
rpc GetAIUsageStats(GetAIUsageStatsRequest) returns (GetAIUsageStatsResponse);

message GetAIUsageStatsRequest {}  // Пользователь из JWT interceptor

message UsageStatInfo {
  string agent_id = 1;       // ID агента
  int32 total_tokens = 2;    // Токены за период
  int32 request_count = 3;   // Количество запросов
  string period_start = 4;   // Начало периода (ISO 8601)
  string agent_name = 5;     // Имя агента
}

message GetAIUsageStatsResponse {
  repeated UsageStatInfo stats = 1;  // Статистика по агентам
  int32 total_tokens = 2;            // Всего токенов
  int32 total_requests = 3;          // Всего запросов
}
```

---

## Data Layer

### GrpcUsageStatsClient

```kotlin
// data/grpc/GrpcUsageStatsClient.kt
package lavender.client.android.data.grpc

import lavender.client.android.domain.billing.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class GrpcUsageStatsClient(
    private val stub: MessengerProto.ChatServiceCoroutineStub
) {
    /**
     * Статистика использования AI.
     * Пользователь определяется из JWT interceptor автоматически.
     */
    suspend fun getUsageStats(): UsageStatsResult = withContext(Dispatchers.IO) {
        val response = stub.getAIUsageStats(
            GetAIUsageStatsRequest.newBuilder().build()
        )
        
        UsageStatsResult(
            stats = response.statsList.map { it.toDomain() },
            totalTokens = response.totalTokens,
            totalRequests = response.totalRequests
        )
    }
}
```

### Domain Models

```kotlin
// domain/billing/UsageStatsModels.kt
package lavender.client.android.domain.billing

data class UsageStat(
    val agentId: String,
    val agentName: String,
    val totalTokens: Int,
    val requestCount: Int,
    val periodStart: String
) {
    val tokensPerRequest: Int
        get() = if (requestCount > 0) totalTokens / requestCount else 0
}

data class UsageStatsResult(
    val stats: List<UsageStat>,
    val totalTokens: Int,
    val totalRequests: Int
) {
    val avgTokensPerRequest: Int
        get() = if (totalRequests > 0) totalTokens / totalRequests else 0
}
```

### Proto Mapper

```kotlin
// data/grpc/UsageStatsMappers.kt
package lavender.client.android.data.grpc

import lavender.client.android.domain.billing.*

fun UsageStatInfoProto.toDomain() = UsageStat(
    agentId = agentId,
    agentName = agentName,
    totalTokens = totalTokens,
    requestCount = requestCount,
    periodStart = periodStart
)
```

---

## Repository Layer

```kotlin
// data/repository/UsageStatsRepository.kt
package lavender.client.android.data.repository

import lavender.client.android.data.grpc.GrpcUsageStatsClient
import lavender.client.android.domain.billing.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

class UsageStatsRepository(private val grpcClient: GrpcUsageStatsClient) {

    private val _stats = MutableStateFlow<UsageStatsResult?>(null)
    val stats: StateFlow<UsageStatsResult?> = _stats

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading

    suspend fun loadStats(): Result<UsageStatsResult> {
        _isLoading.value = true
        return try {
            val result = grpcClient.getUsageStats()
            _stats.value = result
            Result.success(result)
        } catch (e: Exception) {
            Result.failure(e)
        } finally {
            _isLoading.value = false
        }
    }
}
```

---

## UI Layer

### UsageStatsFragment

```kotlin
// ui/ai/billing/UsageStatsFragment.kt
package lavender.client.android.ui.ai.billing

import android.os.Bundle
import android.view.*
import android.widget.*
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import com.google.android.material.appbar.MaterialToolbar
import com.google.android.material.card.MaterialCardView
import kotlinx.coroutines.launch
import lavender.client.android.R
import lavender.client.android.data.grpc.GrpcUsageStatsClient
import lavender.client.android.data.repository.UsageStatsRepository

class UsageStatsFragment : Fragment() {

    private lateinit var repository: UsageStatsRepository
    private lateinit var adapter: UsageStatsAdapter

    // Views
    private lateinit var totalTokensCard: MaterialCardView
    private lateinit var totalRequestsCard: MaterialCardView
    private lateinit var avgTokensCard: MaterialCardView
    private lateinit var recyclerView: RecyclerView
    private lateinit var emptyView: LinearLayout
    private lateinit var progressBar: ProgressBar

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setHasOptionsMenu(true)
        
        val grpcClient = GrpcUsageStatsClient(/* stub */)
        repository = UsageStatsRepository(grpcClient)
    }

    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View {
        return inflater.inflate(R.layout.fragment_usage_stats, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        initViews(view)
        setupRecyclerView()
        observeState()
        loadStats()
    }

    private fun initViews(view: View) {
        totalTokensCard = view.findViewById(R.id.totalTokensCard)
        totalRequestsCard = view.findViewById(R.id.totalRequestsCard)
        avgTokensCard = view.findViewById(R.id.avgTokensCard)
        recyclerView = view.findViewById(R.id.recyclerView)
        emptyView = view.findViewById(R.id.emptyView)
        progressBar = view.findViewById(R.id.progressBar)
        
        // Toolbar
        view.findViewById<MaterialToolbar>(R.id.toolbar)?.let { toolbar ->
            toolbar.title = "Статистика AI"
            toolbar.setNavigationOnClickListener { parentFragmentManager.popBackStack() }
        }
    }

    private fun setupRecyclerView() {
        adapter = UsageStatsAdapter()
        recyclerView.layoutManager = LinearLayoutManager(context)
        recyclerView.adapter = adapter
    }

    private fun observeState() {
        viewLifecycleOwner.lifecycleScope.launch {
            repository.isLoading.collect { loading ->
                progressBar.visibility = if (loading) View.VISIBLE else View.GONE
            }
        }
        
        viewLifecycleOwner.lifecycleScope.launch {
            repository.stats.collect { result ->
                if (result == null) return@collect
                
                if (result.stats.isEmpty()) {
                    recyclerView.visibility = View.GONE
                    emptyView.visibility = View.VISIBLE
                    return@collect
                }
                
                recyclerView.visibility = View.VISIBLE
                emptyView.visibility = View.GONE
                
                // Общая статистика в карточках
                totalTokensCard.findViewById<TextView>(R.id.value).text = 
                    formatNumber(result.totalTokens)
                totalRequestsCard.findViewById<TextView>(R.id.value).text = 
                    formatNumber(result.totalRequests)
                avgTokensCard.findViewById<TextView>(R.id.value).text = 
                    formatNumber(result.avgTokensPerRequest)
                
                // Список по агентам
                adapter.submitList(result.stats)
            }
        }
    }

    private fun loadStats() {
        viewLifecycleOwner.lifecycleScope.launch {
            val result = repository.loadStats()
            result.onFailure {
                Toast.makeText(context, "Ошибка загрузки статистики", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun formatNumber(n: Int): String {
        return when {
            n >= 1_000_000 -> String.format("%.1fM", n / 1_000_000.0)
            n >= 1_000 -> String.format("%.1fK", n / 1_000.0)
            else -> n.toString()
        }
    }
}
```

### UsageStatsAdapter

```kotlin
// ui/ai/billing/UsageStatsAdapter.kt
package lavender.client.android.ui.ai.billing

import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import android.widget.TextView
import androidx.recyclerview.widget.DiffUtil
import androidx.recyclerview.widget.ListAdapter
import androidx.recyclerview.widget.RecyclerView
import lavender.client.android.R
import lavender.client.android.domain.billing.UsageStat

class UsageStatsAdapter : ListAdapter<UsageStat, UsageStatsAdapter.ViewHolder>(DiffCallback) {

    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): ViewHolder {
        val view = LayoutInflater.from(parent.context)
            .inflate(R.layout.item_usage_stat, parent, false)
        return ViewHolder(view)
    }

    override fun onBindViewHolder(holder: ViewHolder, position: Int) {
        holder.bind(getItem(position))
    }

    class ViewHolder(view: View) : RecyclerView.ViewHolder(view) {
        private val agentName: TextView = view.findViewById(R.id.agentName)
        private val tokens: TextView = view.findViewById(R.id.tokens)
        private val requests: TextView = view.findViewById(R.id.requests)
        private val period: TextView = view.findViewById(R.id.period)

        fun bind(stat: UsageStat) {
            agentName.text = stat.agentName.ifEmpty { stat.agentId }
            tokens.text = "${formatNumber(stat.totalTokens)} токенов"
            requests.text = "${stat.requestCount} запросов"
            period.text = formatDate(stat.periodStart)
        }

        private fun formatNumber(n: Int): String {
            return when {
                n >= 1_000_000 -> String.format("%.1fM", n / 1_000_000.0)
                n >= 1_000 -> String.format("%.1fK", n / 1_000.0)
                else -> n.toString()
            }
        }

        private fun formatDate(iso: String): String {
            // "2026-06-20T12:00:00Z" → "20 Jun 2026"
            return try {
                val date = java.time.Instant.parse(iso)
                    .atZone(java.time.ZoneId.systemDefault())
                    .toLocalDate()
                date.format(java.time.format.DateTimeFormatter.ofPattern("d MMM yyyy"))
            } catch (e: Exception) {
                iso
            }
        }
    }

    object DiffCallback : DiffUtil.ItemCallback<UsageStat>() {
        override fun areItemsTheSame(a: UsageStat, b: UsageStat) = a.agentId == b.agentId
        override fun areContentsTheSame(a: UsageStat, b: UsageStat) = a == b
    }
}
```

---

## Layouts

### fragment_usage_stats.xml

```xml
<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:app="http://schemas.android.com/apk/res-auto"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:orientation="vertical">

    <com.google.android.material.appbar.MaterialToolbar
        android:id="@+id/toolbar"
        android:layout_width="match_parent"
        android:layout_height="?attr/actionBarSize"
        app:navigationIcon="?attr/homeAsUpIndicator" />

    <ProgressBar
        android:id="@+id/progressBar"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:layout_gravity="center"
        android:visibility="gone" />

    <!-- Общая статистика -->
    <LinearLayout
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:orientation="horizontal"
        android:padding="16dp">

        <com.google.android.material.card.MaterialCardView
            android:id="@+id/totalTokensCard"
            android:layout_width="0dp"
            android:layout_height="wrap_content"
            android:layout_weight="1"
            android:layout_marginEnd="8dp"
            app:cardCornerRadius="12dp"
            app:cardElevation="2dp">

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:orientation="vertical"
                android:padding="16dp"
                android:gravity="center">

                <TextView
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:text="Токенов"
                    android:textAppearance="?attr/textAppearanceLabelMedium" />

                <TextView
                    android:id="@+id/value"
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:textAppearance="?attr/textAppearanceHeadlineMedium"
                    android:text="0" />
            </LinearLayout>
        </com.google.android.material.card.MaterialCardView>

        <com.google.android.material.card.MaterialCardView
            android:id="@+id/totalRequestsCard"
            android:layout_width="0dp"
            android:layout_height="wrap_content"
            android:layout_weight="1"
            android:layout_marginStart="4dp"
            android:layout_marginEnd="4dp"
            app:cardCornerRadius="12dp"
            app:cardElevation="2dp">

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:orientation="vertical"
                android:padding="16dp"
                android:gravity="center">

                <TextView
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:text="Запросов"
                    android:textAppearance="?attr/textAppearanceLabelMedium" />

                <TextView
                    android:id="@+id/value"
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:textAppearance="?attr/textAppearanceHeadlineMedium"
                    android:text="0" />
            </LinearLayout>
        </com.google.android.material.card.MaterialCardView>

        <com.google.android.material.card.MaterialCardView
            android:id="@+id/avgTokensCard"
            android:layout_width="0dp"
            android:layout_height="wrap_content"
            android:layout_weight="1"
            android:layout_marginStart="8dp"
            app:cardCornerRadius="12dp"
            app:cardElevation="2dp">

            <LinearLayout
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:orientation="vertical"
                android:padding="16dp"
                android:gravity="center">

                <TextView
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:text="Ср. токенов"
                    android:textAppearance="?attr/textAppearanceLabelMedium" />

                <TextView
                    android:id="@+id/value"
                    android:layout_width="wrap_content"
                    android:layout_height="wrap_content"
                    android:textAppearance="?attr/textAppearanceHeadlineMedium"
                    android:text="0" />
            </LinearLayout>
        </com.google.android.material.card.MaterialCardView>
    </LinearLayout>

    <!-- Empty state -->
    <LinearLayout
        android:id="@+id/emptyView"
        android:layout_width="match_parent"
        android:layout_height="match_parent"
        android:gravity="center"
        android:orientation="vertical"
        android:visibility="gone"
        android:padding="32dp">

        <TextView
            android:layout_width="wrap_content"
            android:layout_height="wrap_content"
            android:text="Пока нет данных"
            android:textAppearance="?attr/textAppearanceTitleMedium"
            android:layout_marginBottom="8dp" />

        <TextView
            android:layout_width="wrap_content"
            android:layout_height="wrap_content"
            android:text="Статистика появится после использования AI чатов"
            android:textAppearance="?attr/textAppearanceBodyMedium"
            android:textAlignment="center" />
    </LinearLayout>

    <!-- Список по агентам -->
    <androidx.recyclerview.widget.RecyclerView
        android:id="@+id/recyclerView"
        android:layout_width="match_parent"
        android:layout_height="match_parent"
        android:padding="8dp"
        android:clipToPadding="false" />
</LinearLayout>
```

### item_usage_stat.xml

```xml
<?xml version="1.0" encoding="utf-8"?>
<com.google.android.material.card.MaterialCardView xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:app="http://schemas.android.com/apk/res-auto"
    android:layout_width="match_parent"
    android:layout_height="wrap_content"
    android:layout_margin="8dp"
    app:cardCornerRadius="12dp"
    app:cardElevation="1dp">

    <LinearLayout
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:orientation="vertical"
        android:padding="16dp">

        <TextView
            android:id="@+id/agentName"
            android:layout_width="wrap_content"
            android:layout_height="wrap_content"
            android:textAppearance="?attr/textAppearanceTitleMedium"
            android:text="Agent Name" />

        <LinearLayout
            android:layout_width="match_parent"
            android:layout_height="wrap_content"
            android:orientation="horizontal"
            android:layout_marginTop="8dp">

            <TextView
                android:id="@+id/tokens"
                android:layout_width="0dp"
                android:layout_height="wrap_content"
                android:layout_weight="1"
                android:textAppearance="?attr/textAppearanceBodyMedium"
                android:text="0 токенов" />

            <TextView
                android:id="@+id/requests"
                android:layout_width="0dp"
                android:layout_height="wrap_content"
                android:layout_weight="1"
                android:textAppearance="?attr/textAppearanceBodyMedium"
                android:text="0 запросов" />

            <TextView
                android:id="@+id/period"
                android:layout_width="wrap_content"
                android:layout_height="wrap_content"
                android:textAppearance="?attr/textAppearanceBodySmall"
                android:text="20 Jun 2026" />
        </LinearLayout>
    </LinearLayout>
</com.google.android.material.card.MaterialCardView>
```

---

## Маппинг UI → API

| UI Element | API | Описание |
|------------|-----|----------|
| Карточка "Токенов" | `GetAIUsageStats.total_tokens` | Общее количество токенов |
| Карточка "Запросов" | `GetAIUsageStats.total_requests` | Общее количество запросов |
| Карточка "Ср. токенов" | `total_tokens / total_requests` | Среднее на запрос |
| Список агентов | `GetAIUsageStats.stats[]` | Per-agent breakdown |
| Имя агента | `UsageStatInfo.agent_name` | Название |
| Токены агента | `UsageStatInfo.total_tokens` | Токены за период |
| Запросы агента | `UsageStatInfo.request_count` | Количество |
| Период | `UsageStatInfo.period_start` | Дата начала периода |

---

## Навигация

### Добавить таб "Статистика" в AI экран

```kotlin
// В AiMainFragment:
val tabs = listOf("Мои агенты", "Маркетплейс", "Статистика")

// ViewPager2 adapter:
override fun createFragment(position: Int) = when (position) {
    0 -> MyAgentsFragment()
    1 -> MarketplaceFragment()
    2 -> UsageStatsFragment()
    else -> throw IndexOutOfBoundsException()
}
```

### Или отдельная кнопка в профиле

```kotlin
// В ProfileFragment / Settings:
view.findViewById<View>(R.id.btnUsageStats).setOnClickListener {
    parentFragmentManager.beginTransaction()
        .replace(R.id.container, UsageStatsFragment())
        .addToBackStack(null)
        .commit()
}
```

---

## Чеклист

- [ ] GrpcUsageStatsClient — 1 suspend функция
- [ ] Domain models — UsageStat, UsageStatsResult
- [ ] Proto mapper — UsageStatInfoProto.toDomain()
- [ ] UsageStatsRepository — загрузка, кэш в StateFlow
- [ ] UsageStatsFragment — карточки + список
- [ ] UsageStatsAdapter — DiffUtil + ViewHolder
- [ ] Layouts — fragment_usage_stats.xml, item_usage_stat.xml
- [ ] Навигация — таб "Статистика" в AI экране
- [ ] Empty state — "Пока нет данных"
- [ ] Loading state — ProgressBar
- [ ] Error handling — Toast при ошибке
- [ ] Форматирование чисел — K/M суффиксы
