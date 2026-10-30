
+third_party/tcmalloc/internal/mallocz.proto-tcmalloc.tcmalloc_internal.perftools.profiles"I
HistogramEntry
lower_bound (
upper_bound (
value ("£
SampledTrackerEntry
allocations (
longest_free_range (
objects (
is_hugepage_backed (
density (
reallocation_time_sec ("F
PrioritizationListOccupancy

list_index (
value (J"?
SpanAllocationsTrackerEntry
	num_spans (
value ("í
FreelistEntry
	sizeclass (
bytes (
objects_per_span (
num_spans_requested (
num_spans_returned (
obj_capacity (Z
span_util_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryq
prioritization_list_occupancy (2J.tcmalloc.tcmalloc_internal.perftools.profiles.PrioritizationListOccupancyV
same_span_stats	 (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntrya
span_allocations_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryJ	J
JJRspan_allocations_trackerR!span_completed_lifetime_histogramRspan_lifetime_histogram"Ì
TransferCacheEntry
	sizeclass (
insert_hits (
insert_misses (
insert_non_batch_misses (
remove_hits (
remove_misses (
remove_non_batch_misses (
used (
capacity	 (
max_capacity
 (
insert_object_misses (
remove_object_hits (
remove_object_misses (
frontend_allocations ("≤
CPUCacheEntry
cpu (
used (
unused (
active (
	populated (

underflows (
	overflows (
reclaims (
size_class_resizes	 ("¢
SizeClassCapacityEntry
	sizeclass (
min_capacity (
avg_capacity (
max_capacity (
max_allowed_capacity (
min_last_underflow_ns (
max_last_underflow_ns (
min_last_overflow_ns (
max_last_overflow_ns	 (
max_capacity_misses
 ("K
DynamicSlabEntry
shift (

grow_count (
shrink_count ("º
PageHeapEntry
	span_size (
present (
released (
	num_spans (JJJJ	Ravg_live_age_secsRavg_released_age_secsRlive_age_histRreleased_age_hist"B
HugePageUsageStats
used (
free (
unmapped ("⁄
HugePageTrackerP
type (2B.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageTrackerTypeY
objects (2H.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageTrackerObjectType[
free_pages_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryc
longest_free_range_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntry\
allocations_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryY
lifetime_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryg
 low_occupancy_lifetime_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntrye
long_lived_hugepages_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntry
total_pages (!
num_pages_hugepage_backed (*
"num_free_pages_non_hugepage_backed" (*
"num_used_pages_non_hugepage_backed# (&
num_free_pages_hugepage_backed$ (&
num_used_pages_hugepage_backed% (
num_pages_treated ("
num_pages_collapse_skipped& (1
)num_pages_collapse_skipped_due_to_backoff' (
num_pages_free_swapped (
num_pages_used_swapped  (
num_pages_free_unbacked (
num_pages_used_unbacked! (
num_pages_free_stale( (
num_pages_used_stale) (Y
unbacked_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryX
swapped_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntrya
used_and_swapped_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryb
used_and_unbacked_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntry\
sampled_trackers (2B.tcmalloc.tcmalloc_internal.perftools.profiles.SampledTrackerEntry]
free_swapped_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntry^
free_unbacked_histogram (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryJR allocations_free_pages_histogram"õ
 HugePageFillerSeparateAllocStats
full_huge_pages (
partial_huge_pages (
released_huge_pages (%
partially_released_huge_pages ("¥
HugeRegionDetails

used_bytes (

free_bytes ( 
longest_free_range_bytes (
unbacked_bytes (
total_unbacked_bytes (
backed_fully_free_bytes ("„
HugePageAwareAllocatorSizeStats
min_span_pages (
max_span_pages (
num_spans_allocated (
num_spans_freed (

live_bytes ("
spans_allocated_per_second ("
bytes_allocated_per_second ("Q
MinMaxCurrentStats
	min_bytes (
current_bytes (
	max_bytes ("[
DetailedStat
epoch (
time_offset_ms (
	min_bytes (
	max_bytes ("ä
DetailedStatSeries
	window_ms (
epochs (Q
measurements (2;.tcmalloc.tcmalloc_internal.perftools.profiles.DetailedStat"Ï
FillerStatsSeries
	window_ms (
epochs ("
min_free_pages_interval_ms (
min_free_pages (
min_free_backed_pages (Y
measurements (2C.tcmalloc.tcmalloc_internal.perftools.profiles.FillerStatsDataPoint"s
CollapseErrorStatsN
type (2@.tcmalloc.tcmalloc_internal.perftools.profiles.CollapseErrorType
count ("ó
HugePageTreatmentStats
collapse_eligible (
collapse_attempted (
collapse_succeeded (Z
collapse_errors (2A.tcmalloc.tcmalloc_internal.perftools.profiles.CollapseErrorStats!
treated_pages_subreleased (
collapse_total_time_ms (
collapse_max_time_us (
collapse_backoff_delay ("
collapse_intervals_skipped	 (*
"treated_pages_unbacked_subreleased
 (0
(total_treated_pages_unbacked_subreleased ('
treated_pages_stale_subreleased ("
collapse_donated_succeeded ("“
FillerStatsDataPoint
epoch (
time_offset_ms (U
at_minimum_demand (2:.tcmalloc.tcmalloc_internal.perftools.profiles.FillerStatsU
at_maximum_demand (2:.tcmalloc.tcmalloc_internal.perftools.profiles.FillerStats
min_free_pages (
min_free_backed_pages (
num_pages_subreleased	 (JJJJ
JRtimestamp_msRat_minimum_huge_pagesRat_maximum_huge_pagesRnum_hugepages_brokenRpartial_alloc_pages_subreleased"æ
FillerStats
	num_pages (JJJJJRregular_huge_pagesRdonated_huge_pagesRpartial_released_huge_pagesRreleased_huge_pagesR$used_pages_in_subreleased_huge_pages"£
SkippedSubrelease,
$skipped_subrelease_short_interval_ms	 (+
#skipped_subrelease_long_interval_ms
 ( 
skipped_subrelease_pages (*
"correctly_skipped_subrelease_pages ((
 pending_skipped_subrelease_pages ( 
skipped_subrelease_count (*
"correctly_skipped_subrelease_count ((
 pending_skipped_subrelease_count (
next_peak_interval_ms (JRskipped_subrelease_interval_ms"ò%
HugePageAllocatorInfo

using_hpaa (
using_hpaa_subrelease% (W
filler_usage (2A.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageUsageStatsW
region_usage (2A.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageUsageStatsV
cache_usage (2A.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageUsageStatsV
alloc_usage (2A.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageUsageStats
filler_full_huge_pages (!
filler_partial_huge_pages ("
filler_released_huge_pages (,
$filler_partially_released_huge_pages- (
filler_free_pages
 (
filler_unmapped_bytes (&
filler_hugepageable_used_bytes (!
filler_donated_huge_pages) (
filler_abandoned_pages6 ((
 filler_used_pages_in_subreleased+ (-
%filler_used_pages_in_partial_released, ($
filler_num_pages_subreleased0 (#
filler_num_hugepages_broken1 (1
)filler_num_pages_subreleased_due_to_limit2 (0
(filler_num_hugepages_broken_due_to_limit3 (}
$filler_sparsely_accessed_alloc_stats7 (2O.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageFillerSeparateAllocStats|
#filler_densely_accessed_alloc_stats8 (2O.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageFillerSeparateAllocStats"
use_huge_region_more_often9 (-
%filler_previously_released_huge_pages: (4
,filler_previously_released_backed_huge_pagesH ('
filler_reused_broken_huge_pagesR (u
.sparsely_accessed_completed_lifetime_histogramN (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryt
-densely_accessed_completed_lifetime_histogramO (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntryl
%broken_hugepage_freed_pages_histogramQ (2=.tcmalloc.tcmalloc_internal.perftools.profiles.HistogramEntry%
filler_num_free_pages_swappedI (%
filler_num_used_pages_swappedJ (&
filler_num_used_pages_unbackedK (3
+filler_num_non_free_non_used_unbacked_pagesL (V
filler_tracker* (2>.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageTracker"
min_huge_region_alloc_size (
huge_region_size (]
huge_region_details (2@.tcmalloc.tcmalloc_internal.perftools.profiles.HugeRegionDetails(
 huge_region_low_water_mark_bytesP (b
huge_region_usage_statsS (2A.tcmalloc.tcmalloc_internal.perftools.profiles.MinMaxCurrentStatsh
huge_region_free_backed_statsT (2A.tcmalloc.tcmalloc_internal.perftools.profiles.MinMaxCurrentStatsd
huge_region_usage_historyU (2A.tcmalloc.tcmalloc_internal.perftools.profiles.DetailedStatSeriesj
huge_region_free_backed_historyV (2A.tcmalloc.tcmalloc_internal.perftools.profiles.DetailedStatSeries
huge_cache_time_const' (
cached_huge_page_bytes ("
max_cached_huge_page_bytes (
huge_cache_hit_rate ( 
huge_cache_overflow_rate (
fast_unbacked_bytes (
periodic_unbacked_bytes (a
huge_cache_usage_stats (2A.tcmalloc.tcmalloc_internal.perftools.profiles.MinMaxCurrentStatsc
huge_cache_offpeak_stats (2A.tcmalloc.tcmalloc_internal.perftools.profiles.MinMaxCurrentStatsa
huge_cache_cache_stats (2A.tcmalloc.tcmalloc_internal.perftools.profiles.MinMaxCurrentStats]
huge_cache_history& (2A.tcmalloc.tcmalloc_internal.perftools.profiles.DetailedStatSeries-
%num_huge_address_map_treap_nodes_used (0
(num_huge_address_map_treap_nodes_created (
contiguous_free_bytes (&
num_total_requested_huge_pages (
num_in_use_huge_pages  ("
num_small_allocation_pages! (
num_slack_pages" ( 
largest_allocation_pages# (a
	hpaa_stat$ (2N.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageAwareAllocatorSizeStatsa
filler_stats_timeseries. (2@.tcmalloc.tcmalloc_internal.perftools.profiles.FillerStatsSeriesc
filler_skipped_subrelease/ (2@.tcmalloc.tcmalloc_internal.perftools.profiles.SkippedSubrelease 
num_released_total_pages? (3
+num_released_release_memory_to_system_pages@ (5
-num_released_process_background_actions_pagesA (.
&num_released_soft_limit_exceeded_pagesB (.
&num_released_hard_limit_exceeded_pagesC (o
 filler_huge_page_treatment_statsM (2E.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageTreatmentStatsJ56J	
JJJJ;<J<=J()JDEJEFJ=>J>?JFGJGHJ45Rlifetime_region_usageRfiller_quarantined_huge_pagesRfiller_free_pages_histogramR#filler_longest_free_range_histogramRfiller_allocations_histogramRregion_num_pages_subreleasedR)region_num_pages_subreleased_due_to_limitRhuge_cache_regretRcache_num_hugepages_releasedR)cache_num_hugepages_released_due_to_limitRregion_stats_timeseriesRregion_skipped_subreleaseRcache_stats_timeseriesRcache_skipped_subreleaseRlifetime_based_allocator_stats"Æ
PageAllocatorEntryG
tag (2:.tcmalloc.tcmalloc_internal.perftools.profiles.PageTagTypeO
	page_heap (2<.tcmalloc.tcmalloc_internal.perftools.profiles.PageHeapEntry
min_large_span_size (a
huge_page_allocator (2D.tcmalloc.tcmalloc_internal.perftools.profiles.HugePageAllocatorInfo"É
GwpAsanStats)
!tcmalloc_guarded_sample_parameter (
successful_allocations (
failed_allocations (#
skipped_allocations_noslots ($
skipped_allocations_filtered ($
skipped_allocations_toolarge (
allocated_pages (
quarantine_pages (
high_allocated_pages (
max_allocated_pages (
pages_touched (
total_pages (
mprotect_fallbacks (J	J	
J
JJJRtcmalloc_uaf_check_parameterR#tcmalloc_uaf_check_quarantine_limit"a
SampledProfiles
current_bytes (#
current_fragmentation_bytes (

peak_bytes ("∂
ArenaTypeStats
span (
sampled_allocation (
thread_cache (
stack_trace_table (
filler_tracker (
huge_region (
pagemap (
page_allocator (
guarded_page_allocator	 (
huge_page_metadata
 (
transfer_cache (
	cpu_cache (
test ("Ã2
MalloczEntry
meta ($
scaling_factorn (:0.1B™
fleet_segmentè (
in_use_by_app (
page_heap_freelist (
central_cache_freelist (
per_cpu_cache_freelist ('
sharded_transfer_cache_freelist< (
transfer_cache_freelist (
thread_cache_freelists (
malloc_metadata ()
!malloc_metadata_arena_unavailable> ()
!malloc_metadata_arena_unallocated? (
actual_mem_used	 (
unmapped
 ("
virtual_address_space_used (
pagemap_size" (
	num_spans (
num_spans_created (
num_thread_heaps ( 
num_thread_heaps_created (
num_stack_traces ( 
num_stack_traces_created (
num_table_buckets  (!
num_table_buckets_created! (
tcmalloc_page_size (
tcmalloc_huge_page_size, (
total_resident (
total_mapped (
total_vmpteê (
pagemap_root_residence- (
pagemap_root_sizeé (
peak_backedF (
peak_application_demandG (
percpu_slab_size2 (
percpu_slab_residence3 (W
percpu_vcpu_type9 (2=.tcmalloc.tcmalloc_internal.perftools.profiles.VirtualCpuType
cpus_allowed; (
arena_blocks@ (
arena_freelist_blocksë (N
freelist (2<.tcmalloc.tcmalloc_internal.perftools.profiles.FreelistEntryY
transfer_cache: (2A.tcmalloc.tcmalloc_internal.perftools.profiles.TransferCacheEntryO
	cpu_cache (2<.tcmalloc.tcmalloc_internal.perftools.profiles.CPUCacheEntryO
	page_heap (2<.tcmalloc.tcmalloc_internal.perftools.profiles.PageHeapEntryY
page_allocator1 (2A.tcmalloc.tcmalloc_internal.perftools.profiles.PageAllocatorEntryM
gwp_asan. (2;.tcmalloc.tcmalloc_internal.perftools.profiles.GwpAsanStats
mmap_sys_allocator (
memory_release_failures0 ($
malloc_release_bytes_per_sec (
tcmalloc_per_cpu_caches ('
tcmalloc_max_per_cpu_cache_size (-
%tcmalloc_max_total_thread_cache_bytes (2
*tcmalloc_skip_subrelease_short_interval_nsR (1
)tcmalloc_skip_subrelease_long_interval_nsS (8
/tcmalloc_skip_subrelease_cold_short_interval_nsö (7
.tcmalloc_skip_subrelease_cold_long_interval_nsõ (!
profile_sampling_intervalD ('
madvise_cold_regions_nohugepageK (a
sharded_transfer_cacheN (2A.tcmalloc.tcmalloc_internal.perftools.profiles.TransferCacheEntry&
active_sharded_transfer_cachesO (,
$tcmalloc_release_partial_alloc_pagesV (/
'tcmalloc_release_pages_from_huge_regionY (
active_partitions{ (Z
heap_partitioning| (2?.tcmalloc.tcmalloc_internal.perftools.profiles.PartitioningModeb
size_class_capacityC (2E.tcmalloc.tcmalloc_internal.perftools.profiles.SizeClassCapacityEntry!
dynamic_per_cpu_slab_size^ (U
dynamic_slabH (2?.tcmalloc.tcmalloc_internal.perftools.profiles.DynamicSlabEntry)
!dynamic_slab_madvise_failed_bytesI (
min_large_span_size (
container_rss$ ( 
container_rss_huge_pages% (
container_total_cycles& ($
container_total_instructions' (!
container_milligcu_usageÜ (!
container_milligcu_limitá (
new_hooks_present( (
delete_hooks_present) (!
sampled_new_hooks_present* ($
sampled_delete_hooks_present+ (!
desired_usage_limit_bytes4 (
hard_usage_limit_bytesZ (
soft_limit_hits6 (
hard_limit_hits[ (/
'successful_shrinks_after_soft_limit_hitU (/
'successful_shrinks_after_hard_limit_hit\ ( 
num_released_total_pagesg (3
+num_released_release_memory_to_system_pagesh (5
-num_released_process_background_actions_pagesi (.
&num_released_soft_limit_exceeded_pagesj (.
&num_released_hard_limit_exceeded_pagesk (X
sampled_profiles7 (2>.tcmalloc.tcmalloc_internal.perftools.profiles.SampledProfiles
total_sampled_countT (`
size_class_configb (2E.tcmalloc.tcmalloc_internal.perftools.profiles.SizeClassConfigurationQ
madvisee (2@.tcmalloc.tcmalloc_internal.perftools.profiles.MadvisePreference/
'tcmalloc_resize_size_class_max_capacityl (
text_size_in_bytesq (
min_hot_access_hintt ("
usermode_hugepage_collapsew (
back_small_allocations ("
back_size_threshold_bytesÄ (
cpu_caches_touchedÇ (-
$background_process_sleep_interval_nsÉ (
max_cpu_cache_touchedÑ (
cpu_caches_populatedÖ (,
#tcmalloc_enable_unfiltered_collapseà (&
subrelease_unbacked_hugepagesã (.
%tcmalloc_huge_region_adaptive_releaseå (%
tcmalloc_release_stale_pagesç (/
&tcmalloc_release_drained_slab_metadataí (-
$tcmalloc_madvise_sampled_allocationsì (.
%tcmalloc_cfl_subbucket_prioritizationï (X
arena_type_statsó (2=.tcmalloc.tcmalloc_internal.perftools.profiles.ArenaTypeStats*
!tcmalloc_release_max_filler_pagesò ((
tcmalloc_track_broken_hugepagesú (.
%tcmalloc_preserve_donated_on_collapseù (-
$tcmalloc_collapse_released_hugepagesû (,
#tcmalloc_prefer_backed_filler_pagesü (&
tcmalloc_fix_max_capacity_bug† (JñóJâäJuvJfgJJ89JrsJstJABJEFJPQJQRJmnJcdJWXJBCJJKJXYJ]^J_`J`aJopJvwJdeJpqJxyJyzJz{JJ#$J/0J56J=>JLMJMNJ}~J~JäãJîïJôöR
session_idRproc_self_statmRphysical_cores_usedRselsanRtcmalloc_release_rateR$tcmalloc_skip_subrelease_interval_nsR/tcmalloc_cache_demand_release_short_interval_nsR.tcmalloc_cache_demand_release_long_interval_nsRtcmalloc_shuffle_per_cpu_cachesRtcmalloc_prioritize_spansR&tcmalloc_chunks_for_page_tracker_listsR.separate_allocs_for_few_and_many_objects_spansR(tcmalloc_huge_cache_demand_based_releaseR)tcmalloc_huge_region_demand_based_releaseR&tcmalloc_resize_cpu_cache_size_classesR$tcmalloc_reclaim_idle_per_cpu_cachesRtcmalloc_resize_transfer_cachesR tcmalloc_filler_chunks_per_allocRtcmalloc_use_wider_slabsR-tcmalloc_use_all_buckets_for_few_object_spansR*tcmalloc_configure_size_class_max_capacityR1tcmalloc_dense_trackers_sorted_on_spans_allocatedR2tcmalloc_sparse_trackers_coarse_longest_free_rangeRspan_max_cache_sizeRspan_max_cache_array_sizeRrelease_free_swappedRtcmalloc_num_priority_listsR
hard_limitR
limit_hitsR
other_infoRhuge_page_allocatorR	total_pssRsbrk_sys_allocatorRtransfer_cache_implementationRtcmalloc_partial_transfer_cacheR*tcmalloc_linear_search_length_tracker_listR!use_userspace_collapse_heuristicsRtcmalloc_release_max_cold_pagesR"tcmalloc_release_max_sampled_pagesRhugepage_fragmentation_ratio*W
HugePageTrackerType
UNKNOWN 
REGULAR
DONATED
RELEASED
PARTIAL*]
HugePageTrackerObjectType
UNKNOWN_OBJECTS 
SPARSELY_ACCESSED
DENSELY_ACCESSED*≥
PartitioningMode
PARTITIONING_MODE_UNKNOWN 
PARTITIONING_MODE_DISABLED
PARTITIONING_MODE_NUMA
PARTITIONING_MODE_SECURITY$
 PARTITIONING_MODE_SECURITY_LIGHT*≠
CollapseErrorType
ETYPE_OTHER 
ETYPE_NOMEM

ETYPE_BUSY
ETYPE_INVAL
ETYPE_AGAIN

ETYPE_INTR
ETYPE_ACCES
ETYPE_FAULT
ETYPE_REJECTED*|
PageTagType

NORMAL 
SAMPLED
COLD
	NORMAL_P1

SAMPLED_P1
SAMPLED_OR_COLD
SAMPLED_OR_COLD_P1*U
VirtualCpuType
NONE 
FLAT
NUMA
L3_AWARE
SCHED_STATE
MM*f
TransferCacheImplementationType
UNKNOWN_TRANSFERCACHE 
LIFO
NO_TRANSFERCACHE
RING*Í
SizeClassConfiguration
SIZE_CLASS_UNKNOWN 
SIZE_CLASS_POW2_BELOW_64
SIZE_CLASS_POW2_ONLY
SIZE_CLASS_LEGACY
SIZE_CLASS_LOW_FRAG
SIZE_CLASS_FEWER
SIZE_CLASS_REUSE
SIZE_CLASS_REUSE_V2%
!SIZE_CLASS_REUSE_RELAXED_BELOW_64
SIZE_CLASS_FLEXIBLE	)
%SIZE_CLASS_FLEXIBLE_NO_REUSE_NO_40_56
 
SIZE_CLASS_FLEXIBLE_NO_REUSE*r
MadvisePreference
MADVISE_DONTNEED 
MADVISE_FREE_AND_DONTNEED
MADVISE_FREE_ONLY
MADVISE_NEVERBÚOPEN_TO_OPAQUE_HYBRIDíbeditionspË