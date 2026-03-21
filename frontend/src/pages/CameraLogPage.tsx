import { useState, useEffect } from 'react'
import { Camera, RefreshCw, ChevronLeft, ChevronRight, Image as ImageIcon } from 'lucide-react'
import { wailsAPI } from '../lib/wails'
import type { CameraDetection } from '../types'

export default function CameraLogPage() {
  const [detections, setDetections] = useState<CameraDetection[]>([])
  const [total, setTotal] = useState(0)
  const [limit] = useState(20)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(true)
  const [selectedImage, setSelectedImage] = useState<string | null>(null)

  const loadDetections = async () => {
    try {
      const data = await wailsAPI.getCameraDetections(limit, offset)
      setDetections(data.detections || [])
      setTotal(data.total || 0)
    } catch (error) {
      console.error('Failed to load detections:', error)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadDetections()
  }, [offset])

  const totalPages = Math.ceil(total / limit)
  const currentPage = Math.floor(offset / limit) + 1

  const handlePrevPage = () => {
    if (offset > 0) {
      setOffset(Math.max(0, offset - limit))
    }
  }

  const handleNextPage = () => {
    if (offset + limit < total) {
      setOffset(offset + limit)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600" />
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-gray-900">监测记录</h1>
        <p className="text-gray-500 mt-1">复盘每次摄像头守卫检测到背后有人的截图与 AI 分析理由</p>
      </div>

      {/* Detections List */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Camera className="w-5 h-5 text-primary-600" />
            <h3 className="text-lg font-semibold text-gray-900">检测记录</h3>
          </div>
          <div className="flex items-center gap-3">
            <span className="badge badge-info">共 {total} 条</span>
            <button
              onClick={loadDetections}
              className="btn-secondary btn-sm"
            >
              <RefreshCw className="w-4 h-4" />
              刷新
            </button>
          </div>
        </div>
        <div className="divide-y divide-gray-100">
          {detections.length === 0 ? (
            <div className="p-8 text-center text-gray-500">
              暂无检测记录
            </div>
          ) : (
            detections.map((detection) => (
              <div key={detection.id} className="p-6">
                <div className="flex items-start gap-4">
                  {/* Thumbnail */}
                  <button
                    onClick={() => setSelectedImage(detection.image_url)}
                    className="w-32 h-24 bg-gray-100 rounded-lg flex items-center justify-center overflow-hidden hover:opacity-80 transition-opacity flex-shrink-0"
                  >
                    {detection.image_url ? (
                      <img
                        src={detection.image_url}
                        alt="检测截图"
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <ImageIcon className="w-8 h-8 text-gray-400" />
                    )}
                  </button>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-sm text-gray-500">
                        {detection.detected_at}
                      </span>
                      <span className="px-2 py-0.5 bg-red-100 text-red-700 text-xs rounded-full">
                        检测到有人
                      </span>
                    </div>
                    <div className="bg-gray-50 rounded-lg p-3">
                      <p className="text-sm text-gray-700">
                        <span className="font-medium">AI 分析：</span>
                        {detection.ai_reason || '无分析结果'}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="card-footer flex items-center justify-center gap-4">
            <button
              onClick={handlePrevPage}
              disabled={currentPage === 1}
              className="btn-secondary btn-sm disabled:opacity-50"
            >
              <ChevronLeft className="w-4 h-4" />
              上一页
            </button>
            <span className="text-sm text-gray-600">
              第 {currentPage} / {totalPages} 页
            </span>
            <button
              onClick={handleNextPage}
              disabled={currentPage >= totalPages}
              className="btn-secondary btn-sm disabled:opacity-50"
            >
              下一页
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        )}
      </div>

      {/* Image Lightbox */}
      {selectedImage && (
        <div
          className="fixed inset-0 bg-black/80 flex items-center justify-center z-50"
          onClick={() => setSelectedImage(null)}
        >
          <div className="relative max-w-4xl max-h-[90vh] p-4">
            <button
              onClick={() => setSelectedImage(null)}
              className="absolute top-2 right-2 w-8 h-8 bg-white rounded-full flex items-center justify-center text-gray-900 hover:bg-gray-100"
            >
              ×
            </button>
            <img
              src={selectedImage}
              alt="检测截图"
              className="max-w-full max-h-[80vh] rounded-lg"
              onClick={(e) => e.stopPropagation()}
            />
          </div>
        </div>
      )}
    </div>
  )
}
