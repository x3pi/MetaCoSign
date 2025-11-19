import { memo } from 'react'
import { DOTS, usePagination } from '~/hooks/usePagination'
import { ChevronLeft, ChevronRight } from 'lucide-react'

type PaginationProps = {
  siblingCount?: number
  totalPageCount: number
  onPageChange: (page: number) => void
  currentPage: number
}

const Pagination: React.FC<PaginationProps> = ({ siblingCount = 1, totalPageCount, currentPage, onPageChange }) => {
  const paginationRange = usePagination({ siblingCount, currentPage, totalPageCount })
  return (
    <div className='flex items-center justify-center flex-wrap gap-2 p-4'>
      <button
        type='button'
        disabled={currentPage === 1}
        className={`w-10 h-10 flex items-center justify-center rounded-lg border transition-all
          ${
            currentPage === 1
              ? 'cursor-not-allowed bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-600 border-gray-200 dark:border-gray-700'
              : 'bg-white dark:bg-card text-gray-700 dark:text-gray-300 border-gray-300 dark:border-border hover:bg-blue-50 dark:hover:bg-gray-700 hover:text-blue-600 dark:hover:text-blue-400 hover:border-blue-300 dark:hover:border-blue-600'
          }`}
        onClick={() => onPageChange(currentPage - 1)}
      >
        <ChevronLeft className='w-5 h-5' />
      </button>

      {paginationRange?.map((el: string | number, index: number) => {
        if (el === DOTS) {
          return (
            <span key={index} className='w-10 h-10 flex items-center justify-center text-gray-400 dark:text-gray-500'>
              ...
            </span>
          )
        }

        const isActive = currentPage === el
        return (
          <button
            key={index}
            className={`w-10 h-10 flex items-center justify-center rounded-lg border transition-all font-medium
              ${
                isActive
                  ? 'bg-blue-600 dark:bg-blue-600 text-white border-blue-600 dark:border-blue-600 shadow-md'
                  : 'bg-white dark:bg-card text-gray-700 dark:text-gray-300 border-gray-300 dark:border-border hover:bg-blue-50 dark:hover:bg-blue-900/20 hover:text-blue-600 dark:hover:text-blue-400 hover:border-blue-300 dark:hover:border-blue-600'
              }`}
            onClick={() => onPageChange(el as number)}
          >
            {el}
          </button>
        )
      })}

      <button
        type='button'
        disabled={currentPage === totalPageCount}
        className={`w-10 h-10 flex items-center justify-center rounded-lg border transition-all
          ${
            currentPage === totalPageCount
              ? 'cursor-not-allowed bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-600 border-gray-200 dark:border-gray-700'
              : 'bg-white dark:bg-card text-gray-700 dark:text-gray-300 border-gray-300 dark:border-border hover:bg-blue-50 dark:hover:bg-gray-700 hover:text-blue-600 dark:hover:text-blue-400 hover:border-blue-300 dark:hover:border-blue-600'
          }`}
        onClick={() => onPageChange(currentPage + 1)}
      >
        <ChevronRight className='w-5 h-5' />
      </button>
    </div>
  )
}

export default memo(Pagination)
