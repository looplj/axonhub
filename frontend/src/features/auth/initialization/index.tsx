import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import AuthLayout from '../auth-layout'
import { InitializationForm } from './components/initialization-form'

export default function Initialization() {
  return (
    <AuthLayout>
      <Card className='gap-4'>
        <CardHeader>
          <CardTitle className='text-lg tracking-tight'>Initialize System</CardTitle>
          <CardDescription>
            Welcome to AxonHub! Please set up the system by creating an owner account.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <InitializationForm />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}