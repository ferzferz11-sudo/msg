# AI Marketplace — Android Integration Guide

**Сервер:** v1.3.0.4+ | **Клиент:** v1.3.0.0+ | **Дата:** 2026-06-20

Полный гайд по интеграции AI Marketplace API в Android клиент Lavender Messenger.

---

## Обзор API

| # | Метод | Назначение | Тип |
|---|-------|------------|-----|
| 1 | `ListMarketplaceAgents` | Каталог публичных агентов с поиском | Unary |
| 2 | `GetAIAgentStats` | Статистика агента (установки, рейтинг) | Unary |
| 3 | `GetAIAgentReviews` | Отзывы на агента | Unary |
| 4 | `RateAIAgent` | Оценка агента (1-5 + отзыв) | Unary |
| 5 | `ShareAIAgent` | Генерация share_code | Unary |
| 6 | `InstallAIAgent` | Установка агента по share_code | Unary |
| 7 | `GetAIUsageStats` | Статистика использования (токены) | Unary |

Все методы — Unary RPC (не стриминг), JWT auth через interceptor.

---

## Proto Определения

### RateAIAgent
```protobuf
rpc RateAIAgent(RateAIAgentRequest) returns (RateAIAgentResponse);

message RateAIAgentRequest {
  string agent_id = 1;
  int32 rating = 2;           // 1-5
  string review = 3;          // optional text review
}

message RateAIAgentResponse {
  bool success = 1;
  string error = 2;
  float avg_rating = 3;
  int32 review_count = 4;
}
```

### GetAIAgentReviews
```protobuf
rpc GetAIAgentReviews(GetAIAgentReviewsRequest) returns (GetAIAgentReviewsResponse);

message GetAIAgentReviewsRequest {
  string agent_id = 1;
  int32 limit = 2;            // 0 = default (20)
}

message AgentReviewInfo {
  string user_id = 1;
  int32 rating = 2;
  string review = 3;
  string created_at = 4;
}

message GetAIAgentReviewsResponse {
  repeated AgentReviewInfo reviews = 1;
  float avg_rating = 2;
  int32 review_count = 3;
}
```

### ListMarketplaceAgents
```protobuf
rpc ListMarketplaceAgents(ListMarketplaceAgentsRequest) returns (ListMarketplaceAgentsResponse);

message ListMarketplaceAgentsRequest {
  string query = 1;           // search by name/description/tags
  int32 limit = 2;            // 0 = default (20)
  int32 offset = 3;
}

message ListMarketplaceAgentsResponse {
  repeated AgentInfoV2 agents = 1;
  int32 total = 2;
}
```

### GetAIAgentStats
```protobuf
rpc GetAIAgentStats(GetAIAgentStatsRequest) returns (GetAIAgentStatsResponse);

message GetAIAgentStatsRequest {
  string agent_id = 1;
}

message GetAIAgentStatsResponse {
  int32 install_count = 1;
  float avg_rating = 2;
  int32 review_count = 3;
  int32 total_tokens_used = 4;
}
```

### ShareAIAgent
```protobuf
rpc ShareAIAgent(ShareAIAgentRequest) returns (ShareAIAgentResponse);

message ShareAIAgentRequest {
  string agent_id = 1;
}

message ShareAIAgentResponse {
  bool success = 1;
  string share_code = 2;
  string error = 3;
}
```

### InstallAIAgent
```protobuf
rpc InstallAIAgent(InstallAIAgentRequest) returns (InstallAIAgentResponse);

message InstallAIAgentRequest {
  string share_code = 1;
  string new_name = 2;        // optional rename
}

message InstallAIAgentResponse {
  bool success = 1;
  string agent_id = 2;
  string error = 3;
}
```

### GetAIUsageStats
```protobuf
rpc GetAIUsageStats(GetAIUsageStatsRequest) returns (GetAIUsageStatsResponse);

message GetAIUsageStatsRequest {}

message UsageStatInfo {
  string agent_id = 1;
  int32 total_tokens = 2;
  int32 request_count = 3;
  string period_start = 4;
  string agent_name = 5;
}

message GetAIUsageStatsResponse {
  repeated UsageStatInfo stats = 1;
  int32 total_tokens = 2;
  int32 total_requests = 3;
}
```

---

## Архитектура (三层)

```
UI Layer                    Domain Layer                 Data Layer
┌─────────────────┐        ┌──────────────────┐        ┌────────────────────┐
│ MarketplaceFrag  │◄──────►│ MarketplaceRepo  │◄──────►│ GrpcMarketplace    │
│ AgentDetailFrag  │        │ (свой scope)     │        │ Client             │
│ UsageStatsFrag   │        └──────────────────┘        │ (suspend функции)  │
│ RateDialog       │                                    └────────────────────┘
│ ShareDialog      │
│ InstallDialog    │
└─────────────────┘
```

**Ключевые правила:**
- UI → Repository (через ViewModel)
- Repository → GrpcMarketplaceClient (suspend)
- GrpcMarketplaceClient → gRPC stub
- НЕ вызывать gRPC напрямую из UI/ViewModel

---

## Data Layer

### 1. GrpcMarketplaceClient

```kotlin
// data/grpc/GrpcMarketplaceClient.kt
package lavender.client.android.data.grpc

import lavender.client.android.data.proto.*
import lavender.client.android.domain.ai.MarketplaceAgent
import lavender.client.android.domain.ai.AgentReview
import lavender.client.android.domain.ai.UsageStat
import com.google.protobuf.kotlin.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class GrpcMarketplaceClient(private val client: MessengerProto.ChatServiceCoroutineStub) {

    /**
     * Каталог публичных агентов с поиском.
     * @param query поисковый запрос (пусто = все публичные)
     * @param limit максимальное количество (0 = 20)
     * @param offset смещение для пагинации
     */
    suspend fun listMarketplaceAgents(
        query: String = "",
        limit: Int = 0,
        offset: Int = 0
    ): MarketplaceResult = withContext(Dispatchers.IO) {
        val request = ListMarketplaceAgentsRequest.newBuilder()
            .setQuery(query)
            .setLimit(limit)
            .setOffset(offset)
            .build()
        
        val response = client.listMarketplaceAgents(request)
        
        MarketplaceResult(
            agents = response.agentsList.map { it.toDomain() },
            total = response.total
        )
    }

    /**
     * Статистика агента (установки, рейтинг, отзывы).
     */
    suspend fun getAgentStats(agentId: String): AgentStats = withContext(Dispatchers.IO) {
        val request = GetAIAgentStatsRequest.newBuilder()
            .setAgentId(agentId)
            .build()
        
        val response = client.getAIAgentStats(request)
        
        AgentStats(
            installCount = response.installCount,
            avgRating = response.avgRating,
            reviewCount = response.reviewCount,
            totalTokensUsed = response.totalTokensUsed
        )
    }

    /**
     * Отзывы на агента.
     * @param agentId ID агента
     * @param limit максимальное количество отзывов (0 = 20)
     */
    suspend fun getAgentReviews(
        agentId: String,
        limit: Int = 0
    ): ReviewsResult = withContext(Dispatchers.IO) {
        val request = GetAIAgentReviewsRequest.newBuilder()
            .setAgentId(agentId)
            .setLimit(limit)
            .build()
        
        val response = client.getAIAgentReviews(request)
        
        ReviewsResult(
            reviews = response.reviewsList.map { it.toDomain() },
            avgRating = response.avgRating,
            reviewCount = response.reviewCount
        )
    }

    /**
     * Оценить агента (1-5 + отзыв).
     */
    suspend fun rateAgent(
        agentId: String,
        rating: Int,
        review: String = ""
    ): RateResult = withContext(Dispatchers.IO) {
        require(rating in 1..5) { "Rating must be 1-5" }
        
        val request = RateAIAgentRequest.newBuilder()
            .setAgentId(agentId)
            .setRating(rating)
            .setReview(review)
            .build()
        
        val response = client.rateAIAgent(request)
        
        RateResult(
            success = response.success,
            error = response.error,
            avgRating = response.avgRating,
            reviewCount = response.reviewCount
        )
    }

    /**
     * Генерация share_code для шеринга агента.
     */
    suspend fun shareAgent(agentId: String): ShareResult = withContext(Dispatchers.IO) {
        val request = ShareAIAgentRequest.newBuilder()
            .setAgentId(agentId)
            .build()
        
        val response = client.shareAIAgent(request)
        
        ShareResult(
            success = response.success,
            shareCode = response.shareCode,
            error = response.error
        )
    }

    /**
     * Установка агента по share_code.
     * @param shareCode код из шеринга
     * @param newName опциональное переименование
     */
    suspend fun installAgent(
        shareCode: String,
        newName: String = ""
    ): InstallResult = withContext(Dispatchers.IO) {
        val request = InstallAIAgentRequest.newBuilder()
            .setShareCode(shareCode)
            .setNewName(newName)
            .build()
        
        val response = client.installAIAgent(request)
        
        InstallResult(
            success = response.success,
            agentId = response.agentId,
            error = response.error
        )
    }

    /**
     * Статистика использования AI (токены, запросы).
     * Пользователь определяется из JWT interceptor.
     */
    suspend fun getUsageStats(): UsageStatsResult = withContext(Dispatchers.IO) {
        val request = GetAIUsageStatsRequest.newBuilder().build()
        
        val response = client.getAIUsageStats(request)
        
        UsageStatsResult(
            stats = response.statsList.map { it.toDomain() },
            totalTokens = response.totalTokens,
            totalRequests = response.totalRequests
        )
    }
}
```

### 2. Domain Models

```kotlin
// domain/ai/MarketplaceModels.kt
package lavender.client.android.domain.ai

data class MarketplaceAgent(
    val id: String,
    val name: String,
    val description: String,
    val providerType: String,
    val model: String,
    val toolsEnabled: Boolean,
    val ragEnabled: Boolean,
    val isPreset: Boolean,
    val isPublic: Boolean,
    val avgRating: Float = 0f,
    val installCount: Int = 0
)

data class AgentStats(
    val installCount: Int,
    val avgRating: Float,
    val reviewCount: Int,
    val totalTokensUsed: Int
)

data class AgentReview(
    val userId: String,
    val rating: Int,
    val review: String,
    val createdAt: String
)

data class MarketplaceResult(
    val agents: List<MarketplaceAgent>,
    val total: Int
)

data class ReviewsResult(
    val reviews: List<AgentReview>,
    val avgRating: Float,
    val reviewCount: Int
)

data class RateResult(
    val success: Boolean,
    val error: String,
    val avgRating: Float,
    val reviewCount: Int
)

data class ShareResult(
    val success: Boolean,
    val shareCode: String,
    val error: String
)

data class InstallResult(
    val success: Boolean,
    val agentId: String,
    val error: String
)

data class UsageStat(
    val agentId: String,
    val agentName: String,
    val totalTokens: Int,
    val requestCount: Int,
    val periodStart: String
)

data class UsageStatsResult(
    val stats: List<UsageStat>,
    val totalTokens: Int,
    val totalRequests: Int
)
```

### 3. Proto → Domain Mappers

```kotlin
// data/grpc/MarketplaceMappers.kt
package lavender.client.android.data.grpc

import lavender.client.android.domain.ai.*

fun AgentInfoV2Proto.toDomain() = MarketplaceAgent(
    id = id,
    name = name,
    description = description,
    providerType = providerType,
    model = model,
    toolsEnabled = toolsEnabled,
    ragEnabled = ragEnabled,
    isPreset = isPreset,
    isPublic = isPublic
)

fun AgentReviewInfoProto.toDomain() = AgentReview(
    userId = userId,
    rating = rating,
    review = review,
    createdAt = createdAt
)

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
// data/repository/MarketplaceRepository.kt
package lavender.client.android.data.repository

import lavender.client.android.data.grpc.GrpcMarketplaceClient
import lavender.client.android.domain.ai.*
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

class MarketplaceRepository(private val grpcClient: GrpcMarketplaceClient) {

    private val _marketplaceAgents = MutableStateFlow<List<MarketplaceAgent>>(emptyList())
    val marketplaceAgents: StateFlow<List<MarketplaceAgent>> = _marketplaceAgents

    private val _isLoading = MutableStateFlow(false)
    val isLoading: StateFlow<Boolean> = _isLoading

    private var currentOffset = 0
    private var totalAgents = 0
    private var currentQuery = ""

    /**
     * Загрузить каталог агентов (替换 текущий список).
     */
    suspend fun loadAgents(query: String = ""): Result<MarketplaceResult> {
        _isLoading.value = true
        currentQuery = query
        currentOffset = 0
        
        return try {
            val result = grpcClient.listMarketplaceAgents(query = query, limit = 20, offset = 0)
            _marketplaceAgents.value = result.agents
            totalAgents = result.total
            currentOffset = result.agents.size
            Result.success(result)
        } catch (e: Exception) {
            Result.failure(e)
        } finally {
            _isLoading.value = false
        }
    }

    /**
     * Загрузить следующую страницу (paginated load).
     */
    suspend fun loadMoreAgents(): Result<List<MarketplaceAgent>> {
        if (_isLoading.value) return Result.success(emptyList())
        if (currentOffset >= totalAgents) return Result.success(emptyList())
        
        _isLoading.value = true
        
        return try {
            val result = grpcClient.listMarketplaceAgents(
                query = currentQuery,
                limit = 20,
                offset = currentOffset
            )
            _marketplaceAgents.value = _marketplaceAgents.value + result.agents
            currentOffset += result.agents.size
            Result.success(result.agents)
        } catch (e: Exception) {
            Result.failure(e)
        } finally {
            _isLoading.value = false
        }
    }

    suspend fun getAgentStats(agentId: String): Result<AgentStats> {
        return try {
            Result.success(grpcClient.getAgentStats(agentId))
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    suspend fun getAgentReviews(agentId: String, limit: Int = 20): Result<ReviewsResult> {
        return try {
            Result.success(grpcClient.getAgentReviews(agentId, limit))
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    suspend fun rateAgent(agentId: String, rating: Int, review: String): Result<RateResult> {
        return try {
            Result.success(grpcClient.rateAgent(agentId, rating, review))
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    suspend fun shareAgent(agentId: String): Result<ShareResult> {
        return try {
            Result.success(grpcClient.shareAgent(agentId))
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    suspend fun installAgent(shareCode: String, newName: String = ""): Result<InstallResult> {
        return try {
            Result.success(grpcClient.installAgent(shareCode, newName))
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    suspend fun getUsageStats(): Result<UsageStatsResult> {
        return try {
            Result.success(grpcClient.getUsageStats())
        } catch (e: Exception) {
            Result.failure(e)
        }
    }
}
```

---

## UI Layer

### 1. MarketplaceFragment — Каталог агентов

```kotlin
// ui/ai/marketplace/MarketplaceFragment.kt
package lavender.client.android.ui.ai.marketplace

import android.os.Bundle
import android.view.*
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.GridLayoutManager
import androidx.recyclerview.widget.RecyclerView
import androidx.swiperefreshlayout.widget.SwipeRefreshLayout
import com.google.android.material.search.SearchBar
import com.google.android.material.search.SearchView
import kotlinx.coroutines.launch
import lavender.client.android.R
import lavender.client.android.data.grpc.GrpcMarketplaceClient
import lavender.client.android.data.repository.MarketplaceRepository

class MarketplaceFragment : Fragment() {

    private lateinit var repository: MarketplaceRepository
    private lateinit var adapter: MarketplaceAdapter
    private lateinit var searchBar: SearchBar
    private lateinit var searchView: SearchView
    private lateinit var recyclerView: RecyclerView
    private lateinit var swipeRefresh: SwipeRefreshLayout

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setHasOptionsMenu(true)
        
        // Инициализация (получить из DI или через Activity)
        val grpcClient = GrpcMarketplaceClient(/* stub from GrpcClient */)
        repository = MarketplaceRepository(grpcClient)
    }

    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View {
        return inflater.inflate(R.layout.fragment_marketplace, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        searchBar = view.findViewById(R.id.searchBar)
        searchView = view.findViewById(R.id.searchView)
        recyclerView = view.findViewById(R.id.recyclerView)
        swipeRefresh = view.findViewById(R.id.swipeRefresh)
        
        setupRecyclerView()
        setupSearch()
        setupSwipeRefresh()
        observeState()
        
        // Первая загрузка
        loadAgents()
    }

    private fun setupRecyclerView() {
        adapter = MarketplaceAdapter { agent ->
            // Открыть детали агента
            navigateToAgentDetail(agent.id)
        }
        
        recyclerView.layoutManager = GridLayoutManager(requireContext(), 2)
        recyclerView.adapter = adapter
        
        // Pagination: загрузить больше при прокрутке к концу
        recyclerView.addOnScrollListener(object : RecyclerView.OnScrollListener() {
            override fun onScrolled(rv: RecyclerView, dx: Int, dy: Int) {
                val layoutManager = rv.layoutManager as GridLayoutManager
                val lastVisible = layoutManager.findLastVisibleItemPosition()
                val totalItems = layoutManager.itemCount
                
                if (lastVisible >= totalItems - 5 && !repository.isLoading.value) {
                    viewLifecycleOwner.lifecycleScope.launch {
                        repository.loadMoreAgents()
                    }
                }
            }
        })
    }

    private fun setupSearch() {
        searchView.addTextChangedListener { text ->
            val query = text?.toString() ?: ""
            if (query.length >= 2 || query.isEmpty()) {
                viewLifecycleOwner.lifecycleScope.launch {
                    repository.loadAgents(query)
                }
            }
        }
    }

    private fun setupSwipeRefresh() {
        swipeRefresh.setOnRefreshListener {
            viewLifecycleOwner.lifecycleScope.launch {
                repository.loadAgents()
                swipeRefresh.isRefreshing = false
            }
        }
    }

    private fun observeState() {
        viewLifecycleOwner.lifecycleScope.launch {
            repository.marketplaceAgents.collect { agents ->
                adapter.submitList(agents)
            }
        }
    }

    private fun loadAgents() {
        viewLifecycleOwner.lifecycleScope.launch {
            repository.loadAgents()
        }
    }

    private fun navigateToAgentDetail(agentId: String) {
        // Открыть AgentDetailFragment/Activity
        val fragment = AgentDetailFragment.newInstance(agentId)
        parentFragmentManager.beginTransaction()
            .replace(R.id.container, fragment)
            .addToBackStack(null)
            .commit()
    }
}
```

### 2. AgentDetailFragment — Детали агента

```kotlin
// ui/ai/marketplace/AgentDetailFragment.kt
package lavender.client.android.ui.ai.marketplace

import android.os.Bundle
import android.view.*
import android.widget.*
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import com.google.android.material.chip.Chip
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import kotlinx.coroutines.launch
import lavender.client.android.R
import lavender.client.android.data.repository.MarketplaceRepository

class AgentDetailFragment : Fragment() {

    private lateinit var repository: MarketplaceRepository
    private var agentId: String = ""

    companion object {
        fun newInstance(agentId: String) = AgentDetailFragment().apply {
            arguments = Bundle().apply { putString("agent_id", agentId) }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        agentId = arguments?.getString("agent_id") ?: ""
        setHasOptionsMenu(true)
    }

    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View {
        return inflater.inflate(R.layout.fragment_agent_detail, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        loadAgentDetails(view)
        setupButtons(view)
    }

    private fun loadAgentDetails(view: View) {
        viewLifecycleOwner.lifecycleScope.launch {
            // Загрузить статистику
            val statsResult = repository.getAgentStats(agentId)
            statsResult.onSuccess { stats ->
                view.findViewById<TextView>(R.id.installCount).text = 
                    "Установок: ${stats.installCount}"
                view.findViewById<TextView>(R.id.rating).text = 
                    "Рейтинг: ${String.format("%.1f", stats.avgRating)} (${stats.reviewCount} отзывов)"
            }
            
            // Загрузить отзывы
            val reviewsResult = repository.getAgentReviews(agentId, limit = 10)
            reviewsResult.onSuccess { result ->
                // Показать отзывы в RecyclerView или ListView
                showReviews(result.reviews)
            }
        }
    }

    private fun setupButtons(view: View) {
        // Кнопка "Установить"
        view.findViewById<MaterialButton>(R.id.btnInstall).setOnClickListener {
            installAgent()
        }
        
        // Кнопка "Оценить"
        view.findViewById<MaterialButton>(R.id.btnRate).setOnClickListener {
            showRateDialog()
        }
        
        // Кнопка "Поделиться"
        view.findViewById<MaterialButton>(R.id.btnShare).setOnClickListener {
            shareAgent()
        }
    }

    private fun installAgent() {
        viewLifecycleOwner.lifecycleScope.launch {
            val result = repository.installAgent(agentId)
            result.onSuccess {
                if (it.success) {
                    Toast.makeText(context, "Агент установлен: ${it.agentId}", Toast.LENGTH_SHORT).show()
                    // Обновить UI или закрыть фрагмент
                } else {
                    Toast.makeText(context, "Ошибка: ${it.error}", Toast.LENGTH_SHORT).show()
                }
            }
            result.onFailure {
                Toast.makeText(context, "Ошибка: ${it.message}", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showRateDialog() {
        val dialogView = LayoutInflater.from(context).inflate(R.layout.dialog_rate_agent, null)
        val ratingBar = dialogView.findViewById<RatingBar>(R.id.ratingBar)
        val reviewInput = dialogView.findViewById<EditText>(R.id.reviewInput)
        
        MaterialAlertDialogBuilder(requireContext())
            .setTitle("Оценить агента")
            .setView(dialogView)
            .setPositiveButton("Отправить") { _, _ ->
                val rating = ratingBar.progress
                val review = reviewInput.text.toString()
                
                viewLifecycleOwner.lifecycleScope.launch {
                    val result = repository.rateAgent(agentId, rating, review)
                    result.onSuccess {
                        if (it.success) {
                            Toast.makeText(context, "Спасибо за оценку!", Toast.LENGTH_SHORT).show()
                            loadAgentDetails(requireView()) // Обновить статистику
                        }
                    }
                }
            }
            .setNegativeButton("Отмена", null)
            .show()
    }

    private fun shareAgent() {
        viewLifecycleOwner.lifecycleScope.launch {
            val result = repository.shareAgent(agentId)
            result.onSuccess {
                if (it.success) {
                    // Показать share_code или поделиться через Intent
                    val shareIntent = Intent().apply {
                        action = Intent.ACTION_SEND
                        type = "text/plain"
                        putExtra(Intent.EXTRA_TEXT, "Установи агента в Lavender Messenger: ${it.shareCode}")
                    }
                    startActivity(Intent.createChooser(shareIntent, "Поделиться агентом"))
                }
            }
        }
    }

    private fun showReviews(reviews: List<AgentReview>) {
        // Отобразить отзывы в RecyclerView
        val reviewsAdapter = ReviewsAdapter()
        // ... setup RecyclerView with reviewsAdapter
    }
}
```

### 3. UsageStatsFragment — Статистика использования

```kotlin
// ui/ai/usage/UsageStatsFragment.kt
package lavender.client.android.ui.ai.usage

import android.os.Bundle
import android.view.*
import android.widget.*
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import androidx.recyclerview.widget.RecyclerView
import kotlinx.coroutines.launch
import lavender.client.android.R
import lavender.client.android.data.repository.MarketplaceRepository

class UsageStatsFragment : Fragment() {

    private lateinit var repository: MarketplaceRepository
    private lateinit var adapter: UsageStatsAdapter

    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View {
        return inflater.inflate(R.layout.fragment_usage_stats, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        val recyclerView = view.findViewById<RecyclerView>(R.id.recyclerView)
        adapter = UsageStatsAdapter()
        recyclerView.layoutManager = LinearLayoutManager(context)
        recyclerView.adapter = adapter
        
        loadStats(view)
    }

    private fun loadStats(view: View) {
        viewLifecycleOwner.lifecycleScope.launch {
            val result = repository.getUsageStats()
            result.onSuccess { statsResult ->
                // Показать общую статистику
                view.findViewById<TextView>(R.id.totalTokens).text = 
                    "Всего токенов: ${statsResult.totalTokens}"
                view.findViewById<TextView>(R.id.totalRequests).text = 
                    "Всего запросов: ${statsResult.totalRequests}"
                
                // Показать статистику по агентам
                adapter.submitList(statsResult.stats)
            }
            result.onFailure {
                Toast.makeText(context, "Ошибка загрузки статистики", Toast.LENGTH_SHORT).show()
            }
        }
    }
}
```

### 4. InstallAgentDialog — Установка по share_code

```kotlin
// ui/ai/marketplace/InstallAgentDialog.kt
package lavender.client.android.ui.ai.marketplace

import android.os.Bundle
import android.view.*
import android.widget.*
import androidx.fragment.app.DialogFragment
import androidx.lifecycle.lifecycleScope
import com.google.android.material.button.MaterialButton
import com.google.android.material.textfield.TextInputEditText
import kotlinx.coroutines.launch
import lavender.client.android.R
import lavender.client.android.data.repository.MarketplaceRepository

class InstallAgentDialog : DialogFragment() {

    private lateinit var repository: MarketplaceRepository
    private var onAgentInstalled: ((String) -> Unit)? = null

    companion object {
        fun newInstance(onInstalled: (String) -> Unit) = InstallAgentDialog().apply {
            onAgentInstalled = onInstalled
        }
    }

    override fun onCreateView(inflater: LayoutInflater, container: ViewGroup?, savedInstanceState: Bundle?): View {
        return inflater.inflate(R.layout.dialog_install_agent, container, false)
    }

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        
        val shareCodeInput = view.findViewById<TextInputEditText>(R.id.shareCodeInput)
        val newNameInput = view.findViewById<TextInputEditText>(R.id.newNameInput)
        val installButton = view.findViewById<MaterialButton>(R.id.btnInstall)
        val progressBar = view.findViewById<ProgressBar>(R.id.progressBar)
        
        installButton.setOnClickListener {
            val shareCode = shareCodeInput.text.toString().trim()
            val newName = newNameInput.text.toString().trim()
            
            if (shareCode.isEmpty()) {
                shareCodeInput.error = "Введите код"
                return@setOnClickListener
            }
            
            progressBar.visibility = View.VISIBLE
            installButton.isEnabled = false
            
            viewLifecycleOwner.lifecycleScope.launch {
                val result = repository.installAgent(shareCode, newName)
                
                progressBar.visibility = View.GONE
                installButton.isEnabled = true
                
                result.onSuccess {
                    if (it.success) {
                        Toast.makeText(context, "Агент установлен!", Toast.LENGTH_SHORT).show()
                        onAgentInstalled?.invoke(it.agentId)
                        dismiss()
                    } else {
                        Toast.makeText(context, "Ошибка: ${it.error}", Toast.LENGTH_SHORT).show()
                    }
                }
                result.onFailure {
                    Toast.makeText(context, "Ошибка: ${it.message}", Toast.LENGTH_SHORT).show()
                }
            }
        }
    }
}
```

---

## Интеграция в навигацию

### Добавить Marketplace в AI экран

```kotlin
// В AiMainFragment или аналогичном контейнере:

// Tab 1: Мои агенты (существующий ListAIAgents)
// Tab 2: Маркетплейс (новый ListMarketplaceAgents)
// Tab 3: Статистика (новый GetAIUsageStats)

// ViewPager2 + TabLayout:
val tabTitles = listOf("Мои агенты", "Маркетплейс", "Статистика")
```

### Share/Install через deep link

```kotlin
// В MainActivity обработка deep link:
// lavender://marketplace/install?code=abc123

intent?.data?.let { uri ->
    if (uri.host == "marketplace" && uri.pathSegments.firstOrNull() == "install") {
        val code = uri.getQueryParameter("code") ?: return
        showInstallDialog(code)
    }
}
```

---

## Маппинг UI → API

### Экран "Маркетплейс" (список)

| UI Element | API Method | Описание |
|------------|------------|----------|
| Список агентов | `ListMarketplaceAgents` | Загрузка каталога |
| Поиск | `ListMarketplaceAgents` (query) | Фильтрация по имени/описанию |
| Pull-to-refresh | `ListMarketplaceAgents` | Обновление списка |
| Infinite scroll | `ListMarketplaceAgents` (offset) | Пагинация |
| Карточка агента | — | Отображение name, model, rating, installs |

### Экран "Детали агента"

| UI Element | API Method | Описание |
|------------|------------|----------|
| Статистика | `GetAIAgentStats` | Установки, рейтинг, отзывы |
| Список отзывов | `GetAIAgentReviews` | Отзывы пользователей |
| Кнопка "Установить" | `InstallAIAgent` | Установка в свой список |
| Кнопка "Оценить" | `RateAIAgent` | Отправка оценки 1-5 |
| Кнопка "Поделиться" | `ShareAIAgent` | Генерация share_code |

### Экран "Статистика"

| UI Element | API Method | Описание |
|------------|------------|----------|
| Общие токены | `GetAIUsageStats` | total_tokens |
| Общие запросы | `GetAIUsageStats` | total_requests |
| По агентам | `GetAIUsageStats` | stats[] — per-agent breakdown |

### Диалог "Установка"

| UI Element | API Method | Описание |
|------------|------------|----------|
| Поле share_code | `InstallAIAgent` | Ввод кода |
| Поле имени (optional) | `InstallAIAgent` | Переименование |
| Кнопка "Установить" | `InstallAIAgent` | Вызов API |

---

## Обработка ошибок

| Ошибка | Причина | Действие клиента |
|--------|---------|------------------|
| `UNAVAILABLE` | Сервер недоступен | Показать кэш, retry через 5с |
| `NOT_FOUND` | Агент/код не найден | Показать "Агент не найден" |
| `ALREADY_EXISTS` | Агент уже установлен | Показать "Агент уже в списке" |
| `INVALID_ARGUMENT` | Невалидный rating (не 1-5) | Подсветить поле |
| `PERMISSION_DENIED` | Не авторизован | Предложить войти |

```kotlin
// Обработка ошибок в Repository
suspend fun rateAgent(agentId: String, rating: Int, review: String): Result<RateResult> {
    return try {
        val result = grpcClient.rateAgent(agentId, rating, review)
        if (!result.success) {
            Result.failure(Exception(result.error))
        } else {
            Result.success(result)
        }
    } catch (e: io.grpc.StatusRuntimeException) {
        when (e.status.code) {
            io.grpc.Status.Code.UNAVAILABLE -> Result.failure(Exception("Сервер недоступен"))
            io.grpc.Status.Code.NOT_FOUND -> Result.failure(Exception("Агент не найден"))
            io.grpc.Status.Code.ALREADY_EXISTS -> Result.failure(Exception("Агент уже установлен"))
            io.grpc.Status.Code.INVALID_ARGUMENT -> Result.failure(Exception("Невалидные данные"))
            io.grpc.Status.Code.PERMISSION_DENIED -> Result.failure(Exception("Не авторизован"))
            else -> Result.failure(e)
        }
    }
}
```

---

## Кэширование

| Данные | Стратегия | TTL |
|--------|-----------|-----|
| Каталог агентов | In-memory + Room DB | 5 минут |
| Статистика агента | In-memory | 1 минута |
| Отзывы | In-memory | 5 минут |
| Usage stats | In-memory | 1 минута |
| Share code | Не кэшировать | — |

```kotlin
// Room DAO для кэша маркетплейса
@Dao
interface MarketplaceCacheDao {
    @Query("SELECT * FROM marketplace_agents ORDER BY avg_rating DESC")
    suspend fun getAll(): List<MarketplaceAgentEntity>
    
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertAll(agents: List<MarketplaceAgentEntity>)
    
    @Query("DELETE FROM marketplace_agents")
    suspend fun deleteAll()
}
```

---

## Чеклист интеграции

- [ ] GrpcMarketplaceClient — 7 suspend функций
- [ ] Domain models — MarketplaceAgent, AgentStats, AgentReview, UsageStat
- [ ] Proto mappers — AgentInfoV2Proto.toDomain(), etc.
- [ ] MarketplaceRepository — загрузка, пагинация, кэш
- [ ] MarketplaceFragment — список с поиском и infinite scroll
- [ ] AgentDetailFragment — статистика + отзывы + кнопки
- [ ] UsageStatsFragment — общая статистика + per-agent
- [ ] InstallAgentDialog — ввод share_code
- [ ] RateDialog — рейтинг 1-5 + отзыв
- [ ] ShareAgent — Intent.ACTION_SEND с share_code
- [ ] Error handling — UNAVAILABLE, NOT_FOUND, etc.
- [ ] Deep link — lavender://marketplace/install?code=xxx
- [ ] Room DAO — кэш каталога
- [ ] Навигация — Marketplace таб в AI экране
